package csv_writer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"actor/third/log"
)

// CsvEntry 表示一条待写入的CSV行数据
type CsvEntry struct {
	data     []byte
	capacity int // 跟踪容量以便自动扩容
}

// AsyncCsvWriter 异步批量写入CSV
type AsyncCsvWriter struct {
	fileName    string
	baseDir     string
	window      int64
	startTs     int64
	file        *os.File
	lock        sync.Mutex
	ch          chan *CsvEntry
	chClose     chan error
	once        sync.Once
	isRunning   bool
	channelSize uint
	batchSize   int // 最大批量写入大小
	useWritev   bool
	header      []byte // 可选的CSV头部信息
}

// entryPool 用于复用CsvEntry对象
var entryPool = sync.Pool{
	New: func() any {
		initialCap := 4096
		return &CsvEntry{
			data:     make([]byte, 0, initialCap),
			capacity: initialCap,
		}
	},
}

// NewAsyncCsvWriter 创建异步CSV写入器
func NewAsyncCsvWriter(baseDir string, fileName string, window int64, channelSize int) *AsyncCsvWriter {
	if channelSize < 1 {
		channelSize = 1 // 默认data channel大小为1
	}

	useWritev := isLinux64()

	writer := &AsyncCsvWriter{
		fileName:    fileName,
		baseDir:     baseDir,
		window:      window,
		ch:          make(chan *CsvEntry, channelSize),
		chClose:     make(chan error, 1),
		channelSize: uint(channelSize),
		batchSize:   1024, // writev的最大批量大小
		useWritev:   useWritev,
	}

	// 启动后台写入goroutine
	go writer.processEntries()
	writer.isRunning = true

	return writer
}

// SetHeaders 设置CSV文件的标题行
func (w *AsyncCsvWriter) SetHeaders(headers []string) {
	w.lock.Lock()
	defer w.lock.Unlock()

	// 将字符串数组转换为CSV行
	headerLine := strings.Join(headers, ",") + "\n"
	headerData := []byte(headerLine)

	// 存储标题行数据
	w.header = headerData
}

// Write 异步写入一行CSV数据
func (w *AsyncCsvWriter) Write(data []byte) {
	// 初始化写入器
	w.once.Do(func() {
		if err := w.createNewFile(); err != nil {
			log.Errorf("Failed to create initial writer: %v\n", err)
		}
	})

	// 获取 Entry 对象
	entry := entryPool.Get().(*CsvEntry)

	// 重置并确保容量足够
	entry.data = entry.data[:0]
	if cap(entry.data) < len(data) {
		// 仅在容量不足时分配新内存
		entry.data = make([]byte, 0, len(data)+1024) // 多分配一些，避免频繁扩容
		entry.capacity = len(data) + 1024
	}

	// 复制数据
	entry.data = append(entry.data, data...)

	// 发送到通道
	w.ch <- entry
}

// processEntries 处理所有入队的CSV条目
func (w *AsyncCsvWriter) processEntries() {
	// 根据操作系统选择不同的写入方法
	if w.useWritev {
		w.processBatchEntriesLinux()
	} else {
		w.processBatchEntriesStandard()
	}
}

// processBatchEntriesStandard 标准方式批量写入
func (w *AsyncCsvWriter) processBatchEntriesStandard() {
	const MAX_BATCH = 1024
	entries := make([]*CsvEntry, 0, MAX_BATCH)
	var err error

	for {
		// 获取第一个条目
		entry, ok := <-w.ch
		if !ok || entry == nil {
			break
		}

		entries = append(entries, entry)

		// 尝试批量获取更多条目
		getLimitedBatchEntries(&entries, w.ch, MAX_BATCH-1)

		// 检查是否需要创建新文件
		if err = w.checkAndRotateFile(); err != nil {
			fmt.Printf("Failed to rotate file: %v\n", err)
		}

		// 批量写入
		for _, e := range entries {
			if _, err = w.file.Write(e.data); err != nil {
				fmt.Printf("Failed to write CSV: %v\n", err)
			}

			// 回收Entry对象
			entryPool.Put(e)
		}

		// 清空切片但保留容量
		entries = entries[:0]

		// 每批次后刷新到磁盘
		w.file.Sync()
	}

	// 发送关闭信号
	w.chClose <- err
}

// processBatchEntriesLinux Linux系统使用writev批量写入
func (w *AsyncCsvWriter) processBatchEntriesLinux() {
	const MAX_BATCH = 1024

	var entries [MAX_BATCH]*CsvEntry
	var iovecs [MAX_BATCH]syscall.Iovec
	var err error

	for {
		// 获取第一个条目
		entries[0] = <-w.ch
		if entries[0] == nil {
			break
		}

		iovecs[0].Base = &entries[0].data[0]
		iovecs[0].Len = uint64(len(entries[0].data))

		// 尽可能多地获取条目
		length := len(w.ch)
		if length > MAX_BATCH-1 {
			length = MAX_BATCH - 1
		}

		n := 1
		for n <= length {
			entries[n] = <-w.ch
			if entries[n] == nil {
				break
			}
			iovecs[n].Base = &entries[n].data[0]
			iovecs[n].Len = uint64(len(entries[n].data))
			n++
		}

		// 检查是否需要创建新文件
		if err = w.checkAndRotateFile(); err != nil {
			fmt.Printf("Failed to rotate file: %v\n", err)
		}

		// 使用writev批量写入
		_, err = writev(int(w.file.Fd()), iovecs[:n])

		// 回收所有Entry对象
		for i := 0; i < n; i++ {
			entryPool.Put(entries[i])
			entries[i] = nil
			iovecs[i].Base = nil
		}

		// 定期同步到磁盘
		// w.file.Sync()
	}

	w.chClose <- err
}

// checkAndRotateFile 检查是否需要创建新文件
func (w *AsyncCsvWriter) checkAndRotateFile() error {
	w.lock.Lock()
	defer w.lock.Unlock()

	ts := time.Now().Unix()
	if ts-w.startTs > w.window || w.file == nil {
		return w.createNewFile()
	}
	return nil
}

// createNewFile 创建新的CSV文件
func (w *AsyncCsvWriter) createNewFile() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
	}

	w.startTs = time.Now().Unix()
	filePath := filepath.Join(w.baseDir, fmt.Sprintf("%v@%v.csv", w.fileName, w.startTs))

	// 创建目录
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 打开文件 - 不使用O_SYNC以提高性能，而是定期手动同步
	var err error
	w.file, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// 如果设置了标题行，则写入标题
	if len(w.header) > 0 {
		if _, err := w.file.Write(w.header); err != nil {
			return fmt.Errorf("failed to write headers: %w", err)
		}
	}

	return nil
}

// Close 关闭写入器并刷新所有数据
func (w *AsyncCsvWriter) Close() error {
	if !w.isRunning {
		return nil
	}

	// 发送nil表示关闭信号
	w.ch <- nil

	// 等待处理goroutine完成
	err := <-w.chClose

	// 关闭文件
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.file != nil {
		if err2 := w.file.Close(); err2 != nil && err == nil {
			err = err2
		}
		w.file = nil
	}

	w.isRunning = false
	return err
}

// writev Linux系统调用批量写入
func writev(fd int, iovecs []syscall.Iovec) (uintptr, error) {
	var (
		r uintptr
		e syscall.Errno
	)
	for {
		r, _, e = syscall.Syscall(syscall.SYS_WRITEV, uintptr(fd), uintptr(unsafe.Pointer(&iovecs[0])), uintptr(len(iovecs)))
		if e != syscall.EINTR {
			break
		}
	}
	if e != 0 {
		return r, e
	}
	return r, nil
}

// getLimitedBatchEntries 尝试从通道获取更多条目，但不超过maxCount个
func getLimitedBatchEntries(entries *[]*CsvEntry, ch chan *CsvEntry, maxCount int) {
	count := 0
	for count < maxCount {
		select {
		case entry := <-ch:
			if entry == nil {
				return
			}
			*entries = append(*entries, entry)
			count++
		default:
			return
		}
	}
}

// isLinux64 检查是否是64位Linux系统
func isLinux64() bool {
	return false // 非Linux环境下默认返回false，Linux下编译时可替换为true
}

// LinuxSpecificInit 在Linux平台初始化
func LinuxSpecificInit() {
	// 通过构建标记在Linux平台实现
}
