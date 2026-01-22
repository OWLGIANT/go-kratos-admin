package server

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/tx7do/go-utils/trans"

	"go-wind-admin/app/admin/service/internal/service"

	fileV1 "go-wind-admin/api/gen/go/file/service/v1"
)

func registerFileTransferServiceHandler(srv *http.Server, svc *service.FileTransferService) {
	r := srv.Route("/")

	r.POST("admin/v1/file/upload", _FileTransferService_PostUploadFile_HTTP_Handler(svc))
	r.PUT("admin/v1/file/upload", _FileTransferService_PutUploadFile_HTTP_Handler(svc))

	r.GET("admin/v1/file/download", _FileTransferService_DownloadFile_HTTP_Handler(svc))

	r.POST("admin/v1/ueditor", _FileTransferService_UEditorPostUploadFile_HTTP_Handler(svc))
	r.PUT("admin/v1/ueditor", _FileTransferService_UEditorPutUploadFile_HTTP_Handler(svc))
}

const OperationFileTransferServicePostUploadFile = "/admin.service.v1.FileTransferService/PostUploadFile"
const OperationFileTransferServicePutUploadFile = "/admin.service.v1.FileTransferService/PutUploadFile"

const OperationFileTransferServiceDownloadFile = "/admin.service.v1.FileTransferService/DownloadFile"

const OperationFileTransferServiceUEditorPostUploadFile = "/admin.service.v1.FileTransferService/UEditorPostUploadFile"
const OperationFileTransferServiceUEditorPutUploadFile = "/admin.service.v1.FileTransferService/UEditorPutUploadFile"

func _FileTransferService_PostUploadFile_HTTP_Handler(svc *service.FileTransferService) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		http.SetOperation(ctx, OperationFileTransferServicePostUploadFile)

		var in fileV1.UploadFileRequest
		var err error

		file, header, err := ctx.Request().FormFile("file")
		if err == nil {
			defer file.Close()

			b := new(strings.Builder)
			_, err = io.Copy(b, file)

			in.SourceFileName = trans.Ptr(header.Filename)
			in.Mime = trans.Ptr(header.Header.Get("Content-Type"))
			in.File = []byte(b.String())
		}

		if err = ctx.BindQuery(&in); err != nil {
			return err
		}

		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			aReq := req.(*fileV1.UploadFileRequest)

			var resp *fileV1.UploadFileResponse
			resp, err = svc.UploadFile(ctx, aReq)
			in.File = nil

			return resp, err
		})

		// 逻辑处理，取数据
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}

		reply := out.(*fileV1.UploadFileResponse)

		return ctx.Result(200, reply)
	}
}

func _FileTransferService_PutUploadFile_HTTP_Handler(svc *service.FileTransferService) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		http.SetOperation(ctx, OperationFileTransferServicePutUploadFile)

		var in fileV1.UploadFileRequest
		var err error

		file, header, err := ctx.Request().FormFile("file")
		if err == nil {
			defer file.Close()

			b := new(strings.Builder)
			_, err = io.Copy(b, file)

			in.SourceFileName = trans.Ptr(header.Filename)
			in.Mime = trans.Ptr(header.Header.Get("Content-Type"))
			in.File = []byte(b.String())
		}

		if err = ctx.BindQuery(&in); err != nil {
			return err
		}

		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			aReq := req.(*fileV1.UploadFileRequest)

			var resp *fileV1.UploadFileResponse
			resp, err = svc.UploadFile(ctx, aReq)
			in.File = nil

			return resp, err
		})

		// 逻辑处理，取数据
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}

		reply := out.(*fileV1.UploadFileResponse)

		return ctx.Result(200, reply)
	}
}

func _FileTransferService_DownloadFile_HTTP_Handler(svc *service.FileTransferService) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		http.SetOperation(ctx, OperationFileTransferServiceDownloadFile)

		var in fileV1.DownloadFileRequest
		var err error

		if err = ctx.BindQuery(&in); err != nil {
			return err
		}

		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			aReq := req.(*fileV1.DownloadFileRequest)
			var resp *fileV1.DownloadFileResponse
			resp, err = svc.DownloadFile(ctx, aReq)
			return resp, err
		})

		// 逻辑处理，取数据
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}

		reply := out.(*fileV1.DownloadFileResponse)
		rw := ctx.Response()
		if rw == nil {
			return ctx.Result(500, "response writer not available")
		}

		data := reply.GetFile()
		if len(data) == 0 {
			// 若没有文件字节，交由框架默认处理（保持原行为）
			return ctx.Result(200, reply)
		}

		// 基本头部
		mime := reply.GetMime()
		if mime == "" {
			mime = "application/octet-stream"
		}
		rw.Header().Set("Content-Type", mime)

		filename := reply.GetSourceFileName()
		if filename == "" {
			filename = "file"
		}

		var disposition string
		if in.GetDisposition() != "" {
			disposition = in.GetDisposition()
		} else {
			disposition = "attachment; filename=\"" + filename + "\""
		}
		rw.Header().Set("Content-Disposition", disposition)
		rw.Header().Set("Accept-Ranges", "bytes")

		// 使用 bytes.Reader 以便支持高效的部分读取/流式写入
		reader := bytes.NewReader(data)
		total := int64(len(data))

		// 检查请求是否带 Range 头（仅支持单区间，格式 bytes=start-end 或 bytes=start-）
		rangeHeader := ctx.Request().Header.Get("Range")
		if strings.HasPrefix(rangeHeader, "bytes=") {
			r := strings.TrimPrefix(rangeHeader, "bytes=")
			parts := strings.SplitN(r, "-", 2)
			if len(parts) != 2 {
				return ctx.Result(400, "invalid Range header")
			}

			start, err1 := strconv.ParseInt(parts[0], 10, 64)
			if err1 != nil || start < 0 {
				return ctx.Result(400, "invalid Range start")
			}

			var end int64
			if parts[1] == "" {
				end = total - 1
			} else {
				end, err1 = strconv.ParseInt(parts[1], 10, 64)
				if err1 != nil || end < start {
					return ctx.Result(400, "invalid Range end")
				}
			}

			if start >= total {
				return ctx.Result(416, "requested range not satisfiable")
			}
			if end >= total {
				end = total - 1
			}

			length := end - start + 1
			// 设置部分响应头
			rw.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(total, 10))
			rw.Header().Set("Content-Length", strconv.FormatInt(length, 10))

			// 返回 206 并写入指定区间
			rw.WriteHeader(206)
			if _, err = reader.Seek(start, io.SeekStart); err != nil {
				return ctx.Result(500, err.Error())
			}
			if _, err = io.CopyN(rw, reader, length); err != nil {
				return ctx.Result(500, err.Error())
			}
			return nil
		}

		// 无 Range，返回完整内容（200）
		rw.Header().Set("Content-Length", strconv.FormatInt(total, 10))
		rw.WriteHeader(200)
		if _, err = io.Copy(rw, reader); err != nil {
			return ctx.Result(500, err.Error())
		}
		return nil
	}
}

func _FileTransferService_UEditorPostUploadFile_HTTP_Handler(svc *service.FileTransferService) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		http.SetOperation(ctx, OperationFileTransferServiceUEditorPostUploadFile)

		var in fileV1.UEditorUploadRequest
		var err error

		file, header, err := ctx.Request().FormFile("file")
		if err == nil {
			defer file.Close()

			b := new(strings.Builder)
			_, err = io.Copy(b, file)

			in.SourceFileName = trans.Ptr(header.Filename)
			in.Mime = trans.Ptr(header.Header.Get("Content-Type"))
			in.File = []byte(b.String())
		}

		if err = ctx.BindQuery(&in); err != nil {
			return err
		}

		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			var resp *fileV1.UEditorUploadResponse

			resp, err = svc.UEditorUploadFile(ctx, req.(*fileV1.UEditorUploadRequest))
			in.File = nil

			return resp, err
		})

		// 逻辑处理，取数据
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}

		reply := out.(*fileV1.UEditorUploadResponse)

		return ctx.Result(200, reply)
	}
}

func _FileTransferService_UEditorPutUploadFile_HTTP_Handler(svc *service.FileTransferService) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		http.SetOperation(ctx, OperationFileTransferServiceUEditorPutUploadFile)

		var in fileV1.UEditorUploadRequest
		var err error

		file, header, err := ctx.Request().FormFile("file")
		if err == nil {
			defer file.Close()

			b := new(strings.Builder)
			_, err = io.Copy(b, file)

			in.SourceFileName = trans.Ptr(header.Filename)
			in.Mime = trans.Ptr(header.Header.Get("Content-Type"))
			in.File = []byte(b.String())
		}

		if err = ctx.BindQuery(&in); err != nil {
			return err
		}

		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			var resp *fileV1.UEditorUploadResponse

			resp, err = svc.UEditorUploadFile(ctx, req.(*fileV1.UEditorUploadRequest))
			in.File = nil

			return resp, err
		})

		// 逻辑处理，取数据
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}

		reply := out.(*fileV1.UEditorUploadResponse)

		return ctx.Result(200, reply)
	}
}
