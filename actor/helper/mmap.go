package helper

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"actor/third/log"
	"github.com/go-mmap/mmap"
)

// const MAX_SIZE = 128 * 1024 / 4

const MAX_SIZE = 128 * 1024 * 1024 // 128 MB

// The format of the mmap file is as follows:
// PrevFileName----------00001234timestamp{xxxx}00001234timestamp{xxxx}00001234timestamp{xxxx}00000000NextFileName----------
const STRLEN_PREFIX_LEN = 4 // Length of the length prefix in bytes (int 32)
const TSNS_LEN = 8
const FILENAME_LEN = 60 // Length reserved for the filename
var LINE_PREFIX_LEN = STRLEN_PREFIX_LEN + TSNS_LEN
var SECTION_PREFIX_LEN = FILENAME_LEN * 2
var SECTION_SUFFIX_LEN = STRLEN_PREFIX_LEN

type Mmap struct {
	Data                   *mmap.File // mmap数据
	file                   *os.File
	lock                   sync.Mutex
	fileNameBase           string
	currentSectionFileName string
	dir                    string
	isClosedByExt          bool
}

func NewMmap(dir, fileNameBase string) (m *Mmap, err error) {
	if len(fileNameBase) > FILENAME_LEN-20 { // name@startNs_endNs
		return nil, fmt.Errorf("fileNameBase is too long, must be less than 50 characters")
	}

	m = &Mmap{
		fileNameBase: fileNameBase,
		dir:          dir,
	}
	m.initSectionForWrite(dir, getFileName(m.fileNameBase))

	go func() {
		// ticker := time.NewTicker(10 * time.Minute)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if m.Data != nil {
				if err := m.Data.Sync(); err != nil {
					log.Errorf("could not sync mmap file: %+v", err)
					return
				}
			}
		}
	}()

	return
}

func newMmap(nextSectionName string) (mmapData *mmap.File, f *os.File, err error) {
	log.Info("Creating new section:", nextSectionName)

	f, err = os.OpenFile(nextSectionName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Errorf("Error opening file: %v", err)
		return
	}
	// Truncate the file to maxSize
	if err = f.Truncate(int64(MAX_SIZE)); err != nil {
		log.Errorf("Error truncating file: %v", err)
		return
	}

	mmapData, err = mmap.OpenFile(f.Name(), mmap.Read|mmap.Write)
	if err != nil {
		log.Errorf("could not open mmap file: %+v", err)
	}
	return
}

func (m *Mmap) Write(line []byte, ts int64) (bytes int, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	//
	// if len(line) > MAX_SIZE-2*STRLEN_PREFIX_LEN-2*FILENAME_LEN {
	// 	return 0, fmt.Errorf("line is too long, must be less than %d bytes", MAX_SIZE)
	// }
	//
	maxPossibleSizeToWrite := LINE_PREFIX_LEN + len(line) + SECTION_SUFFIX_LEN
	currentSize, err := m.Data.Seek(0, io.SeekCurrent) // could have error closed
	if int(currentSize)+maxPossibleSizeToWrite > MAX_SIZE || err != nil {
		nextName := m.closeInner()
		log.Infof("Current section size exceeded, creating new section")
		err = m.initSectionForWrite(m.dir, nextName)
		log.Infof("New section initialized: %s", nextName)
		if err != nil {
			log.Errorf("could not create new mmap section: %+v", err)
			return
		}
	}
	// Write the line to the mmap file
	prefixBuf := make([]byte, STRLEN_PREFIX_LEN)
	binary.LittleEndian.PutUint32(prefixBuf, uint32(len(line)))
	_, err = m.Data.Write(prefixBuf)
	if err != nil {
		log.Errorf("could not write to mmap file: %+v", err)
		return
	}
	// Write the timestamp
	tsBuf := make([]byte, TSNS_LEN)
	binary.LittleEndian.PutUint64(tsBuf, uint64(ts))
	_, err = m.Data.Write(tsBuf)
	if err != nil {
		log.Errorf("could not write to mmap file: %+v", err)
		return
	}
	bytes, err = m.Data.Write(line)
	if err != nil {
		log.Errorf("could not write to mmap file: %+v", err)
		return
	}
	return
}

func (m *Mmap) Close() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.closeInner()
}

func (m *Mmap) closeInner() (nextFileName string) {
	m.Data.Write(make([]byte, STRLEN_PREFIX_LEN)) // len=0 means eof
	nextFileName = getFileName(m.fileNameBase)
	m.Data.Seek(FILENAME_LEN, io.SeekStart)
	m.Data.Write([]byte(nextFileName))
	//
	m.Data.Sync()
	m.file.Close()
	m.Data.Close()
	return
}

func (m *Mmap) initSectionForWrite(dir, nextSectionName string) (err error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Errorf("could not create directory %s: %+v", dir, err)
			return fmt.Errorf("could not create directory %s: %w", dir, err)
		}
	}

	m.Data, m.file, err = newMmap(dir + "/" + nextSectionName)
	if err != nil {
		log.Errorf("could not create new mmap section: %+v", err)
	}
	//
	buf := make([]byte, FILENAME_LEN*2)
	copy(buf, []byte(m.currentSectionFileName))
	m.Data.Write(buf)
	m.currentSectionFileName = nextSectionName
	return
}

func getFileName(fileNameBase string) string {
	slashIdx := strings.LastIndex(fileNameBase, "/")
	if slashIdx != -1 {
		fileNameBase = fileNameBase[slashIdx+1:] // Remove path if present
	}
	return fmt.Sprintf("%s@%d", fileNameBase, time.Now().UnixNano())
}

// ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
type MmapReplay struct {
	Data                         *mmap.File // mmap数据
	file                         *os.File
	cb                           func(line []byte, ts int64)
	once                         sync.Once
	currentSectionFileNameReplay string
	nextSectionFileNameReplay    string
	dir                          string
}

func NewMmapReplay(dir, fileNameBase string, fromTsNs int64, cb func(line []byte, ts int64)) (res *MmapReplay, err error) {
	res = &MmapReplay{
		cb:                           cb,
		currentSectionFileNameReplay: fmt.Sprintf("%s@%d", fileNameBase, fromTsNs),
		nextSectionFileNameReplay:    fmt.Sprintf("%s@%d", fileNameBase, fromTsNs),
		dir:                          dir,
	}

	log.Infof("Creating new MmapReplay with fileNameBase: %s", fileNameBase)
	return res, nil
}

func newMmapReplay(nextFileName string) (mmapData *mmap.File, f *os.File, err error) {
	log.Info("Expected next section:", nextFileName)

	f, err = os.OpenFile(nextFileName, os.O_RDONLY, 0666)
	if err != nil {
		log.Errorf("Error opening file: %v", err)
		return nil, nil, fmt.Errorf("could not open file %s: %w", nextFileName, err)
	}
	mmapData, err = mmap.OpenFile(f.Name(), mmap.Read)
	if err != nil {
		log.Errorf("could not open mmap file: %+v", err)
	}
	return
}

func (m *MmapReplay) RunReplay(toTsNs int64) (err error) {
	isFirstTime := false
	m.once.Do(func() {
		isFirstTime = true
	})
	if !isFirstTime {
		panic("MmapReplay can only be run once")
	}

	lineStartPos := int64(0)

	for {
		if lineStartPos == 0 {
			if err = m.initNextSectionForRead(); err != nil {
				log.Infof("No more sections to read, exiting replay: %+v", err)
				return nil
			}
			log.Infof("Initialized section: %s", m.currentSectionFileNameReplay)
			lineStartPos, _ = m.Data.Seek(int64(SECTION_PREFIX_LEN), io.SeekStart)
		}

		// Len /////////////////////////////
		strLenBuf := make([]byte, STRLEN_PREFIX_LEN)
		if _, err := m.Data.ReadAt(strLenBuf, lineStartPos); err != nil {
			return fmt.Errorf("could not read strLen: %w", err)
		}
		strLen := binary.LittleEndian.Uint32(strLenBuf)
		if strLen == 0 {
			lineStartPos = 0
			log.Infof("Reached end of section %s, next section expected %s", m.currentSectionFileNameReplay, m.nextSectionFileNameReplay)
			continue
		}

		log.Debugf("Reading line of length:%d %d %d", lineStartPos, len(strLenBuf), strLen)
		// Move Ptr /////////////////////////
		tsStart, _ := m.Data.Seek(STRLEN_PREFIX_LEN, io.SeekCurrent)

		// Ts ///////////////////////////////
		tsBuf := make([]byte, TSNS_LEN)
		if _, err := m.Data.ReadAt(tsBuf, tsStart); err != nil {
			return fmt.Errorf("could not read ts: %w", err)
		}
		ts := int64(binary.LittleEndian.Uint64(tsBuf))
		if ts >= toTsNs && toTsNs > 0 {
			log.Infof("Reached end of replay at timestamp: %d, stopping replay", ts)
			return nil
		}

		log.Debugf("Reading timestamp: %d %d %d", tsStart, len(tsBuf), ts)

		// Move Ptr /////////////////////////
		lineStart, _ := m.Data.Seek(TSNS_LEN, io.SeekCurrent)
		// Line /////////////////////////////
		lineBuf := make([]byte, strLen)
		if _, err := m.Data.ReadAt(lineBuf, lineStart); err != nil {
			return fmt.Errorf("could not read line: %w", err)
		}
		m.cb(lineBuf, ts)
		// Move Ptr /////////////////////////
		lineStartPos, _ = m.Data.Seek(int64(strLen), io.SeekCurrent)
	}
}

func (m *MmapReplay) initNextSectionForRead() (err error) {
	if m.Data != nil {
		m.Data.Close()
	}
	if m.file != nil {
		m.file.Close()
	}
	if m.nextSectionFileNameReplay != "" {
		m.Data, m.file, err = newMmapReplay(m.dir + "/" + m.nextSectionFileNameReplay)
	}
	log.Info("next file name:", m.nextSectionFileNameReplay)
	if err != nil || m.nextSectionFileNameReplay == "" {
		log.Errorf("could not create new mmap section: %+v, try find next", err)
		m.nextSectionFileNameReplay = findNextSectionFileName(m.dir, m.currentSectionFileNameReplay)
		if m.nextSectionFileNameReplay == "" {
			return fmt.Errorf("could not find next section file name after: %s", m.currentSectionFileNameReplay)
		}
		m.Data, m.file, err = newMmapReplay(m.dir + "/" + m.nextSectionFileNameReplay)
		if err != nil {
			return err
		}
	}
	//
	m.currentSectionFileNameReplay = m.nextSectionFileNameReplay
	prevAndNextNameBuf := make([]byte, FILENAME_LEN*2)
	if _, err := m.Data.ReadAt(prevAndNextNameBuf, 0); err != nil {
		log.Errorf("could not read prev and next file names: %+v", err)
		return fmt.Errorf("could not read prev and next file names: %w", err)
	}

	m.nextSectionFileNameReplay = trimNulls(string(prevAndNextNameBuf[FILENAME_LEN:]))
	return
}

func trimNulls(s string) string {
	return strings.TrimRight(s, "\x00")
}

func findNextSectionFileName(dir, currentSectionFileName string) (nextFileName string) {
	log.Info("Searching for next section file name after:", currentSectionFileName)

	var err error
	atIdx := strings.LastIndex(currentSectionFileName, "@")
	if atIdx == -1 {
		log.Errorf("current section file name does not contain '@': %s", currentSectionFileName)
		return ""
	}
	currentFileStartTime := currentSectionFileName[atIdx+1:]
	currentFileStartNs, err := strconv.ParseInt(currentFileStartTime, 10, 64)
	if err != nil {
		log.Errorf("could not parse current file start time: %v", err)
		return ""
	}

	nextFileNs := int64(0)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), currentSectionFileName[:atIdx+1]) {
			fileStartTime := filepath.Base(path)[atIdx+1:]
			fileStartNs, _ := strconv.ParseInt(fileStartTime, 10, 64)
			if fileStartNs > currentFileStartNs {
				if nextFileNs == 0 || fileStartNs < nextFileNs {
					nextFileNs = fileStartNs
				}
			}
		}
		return nil
	})

	if nextFileNs == 0 {
		return ""
	}
	nextFileName = fmt.Sprintf("%s@%d", currentSectionFileName[:atIdx], nextFileNs)
	log.Info("Next section file name found:", nextFileName)
	return nextFileName
}

// log.Init("log", log.DebugLevel,
// 	log.SetStdout(true),
// 	log.SetCaller(true),
// 	log.SetMaxBackups(10),
// 	log.SetLogger(log.LoggerZaplog),
// )
// log.PanicIfErrorUnderUTF = false

// mmapVerify, _ := helper.NewMmap("./verify", "binance_streams")

// MmapReplay, _ := helper.NewMmapReplay("./ssss", "binance_streams", 0, func(line []byte, ts int64) {
// 	log.Debugf("Replay line: %s, ts: %d\n", string(line), ts)
// 	mmapVerify.Write(line, ts)
// })

// MmapReplay.RunReplay(0)
// return
