package service

import (
	"context"
	"io"
	"net/http"
	"path"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"
	fileV1 "go-wind-admin/api/gen/go/file/service/v1"

	"go-wind-admin/pkg/oss"
)

type FileTransferService struct {
	adminV1.FileTransferServiceHTTPServer

	log *log.Helper

	mc *oss.MinIOClient
}

func NewFileTransferService(ctx *bootstrap.Context, mc *oss.MinIOClient) *FileTransferService {
	return &FileTransferService{
		log: ctx.NewLoggerHelper("file-transfer/service/admin-service"),
		mc:  mc,
	}
}

func (s *FileTransferService) UploadFile(ctx context.Context, req *fileV1.UploadFileRequest) (*fileV1.UploadFileResponse, error) {
	if req.File == nil {
		return nil, fileV1.ErrorUploadFailed("unknown fileData")
	}

	if req.BucketName == nil {
		req.BucketName = trans.Ptr(s.mc.ContentTypeToBucketName(req.GetMime()))
	}
	if req.ObjectName == nil {
		req.ObjectName = trans.Ptr(req.GetSourceFileName())
	}

	downloadUrl, err := s.mc.UploadFile(ctx, req.GetBucketName(), req.GetObjectName(), req.GetFile())
	return &fileV1.UploadFileResponse{
		Url: downloadUrl,
	}, err
}

// downloadFileFromURL 从指定的 URL 下载文件内容
func (s *FileTransferService) downloadFileFromURL(ctx context.Context, downloadUrl string) (*fileV1.DownloadFileResponse, error) {
	if downloadUrl == "" {
		return nil, fileV1.ErrorDownloadFailed("empty download url")
	}

	// 如果需要支持断点续传，可在此构造请求并设置 Range 头
	httpReq, err := http.NewRequestWithContext(ctx, "GET", downloadUrl, nil)
	if err != nil {
		return nil, fileV1.ErrorDownloadFailed(err.Error())
	}
	// 示例：如果你要设置 Range（可选）
	// httpReq.Header.Set("Range", "bytes=100-")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fileV1.ErrorDownloadFailed(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fileV1.ErrorDownloadFailed("unexpected status: " + resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fileV1.ErrorDownloadFailed(err.Error())
	}

	return &fileV1.DownloadFileResponse{
		Content: &fileV1.DownloadFileResponse_File{
			File: data,
		},
	}, nil
}

// DownloadFile 下载文件
func (s *FileTransferService) DownloadFile(ctx context.Context, req *fileV1.DownloadFileRequest) (*fileV1.DownloadFileResponse, error) {
	switch req.Selector.(type) {
	case *fileV1.DownloadFileRequest_FileId:
		return nil, fileV1.ErrorDownloadFailed("unsupported file ID download")

	case *fileV1.DownloadFileRequest_StorageObject:
		return s.mc.DownloadFile(ctx, req)

	case *fileV1.DownloadFileRequest_DownloadUrl:
		return s.downloadFileFromURL(ctx, req.GetDownloadUrl())

	default:
		return nil, fileV1.ErrorDownloadFailed("unknown download selector")
	}
}

func (s *FileTransferService) UEditorUploadFile(ctx context.Context, req *fileV1.UEditorUploadRequest) (*fileV1.UEditorUploadResponse, error) {
	//s.log.Infof("上传文件： %s", req.GetFile())

	if req.File == nil {
		return nil, fileV1.ErrorUploadFailed("unknown file")
	}

	var bucketName string
	switch req.GetAction() {
	default:
		fallthrough
	case fileV1.UEditorAction_uploadFile.String():
		bucketName = "files"
	case fileV1.UEditorAction_uploadImage.String(), fileV1.UEditorAction_uploadScrawl.String(), fileV1.UEditorAction_catchImage.String():
		bucketName = "images"
	case fileV1.UEditorAction_uploadVideo.String():
		bucketName = "videos"
	}

	downloadUrl, err := s.mc.UploadFile(ctx, bucketName, req.GetSourceFileName(), req.GetFile())
	if err != nil {
		return &fileV1.UEditorUploadResponse{
			State: trans.Ptr(err.Error()),
		}, err
	}

	return &fileV1.UEditorUploadResponse{
		State:    trans.Ptr(StateOK),
		Original: trans.Ptr(req.GetSourceFileName()),
		Title:    trans.Ptr(req.GetSourceFileName()),
		Url:      trans.Ptr(downloadUrl),
		Size:     trans.Ptr(int32(len(req.GetFile()))),
		Type:     trans.Ptr(path.Ext(req.GetSourceFileName())),
	}, nil
}
