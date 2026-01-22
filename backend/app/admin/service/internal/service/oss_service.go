package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"
	fileV1 "go-wind-admin/api/gen/go/file/service/v1"

	"go-wind-admin/pkg/oss"
)

type OssService struct {
	adminV1.OssServiceHTTPServer

	log *log.Helper

	mc *oss.MinIOClient
}

func NewOssService(ctx *bootstrap.Context, mc *oss.MinIOClient) *OssService {
	return &OssService{
		log: ctx.NewLoggerHelper("oss/service/admin-service"),
		mc:  mc,
	}
}

func (s *OssService) GetUploadPresignedUrl(ctx context.Context, req *fileV1.GetUploadPresignedUrlRequest) (*fileV1.GetUploadPresignedUrlResponse, error) {
	return s.mc.GetUploadPresignedUrl(ctx, req)
}

func (s *OssService) ListOssFile(ctx context.Context, req *fileV1.ListOssFileRequest) (*fileV1.ListOssFileResponse, error) {
	return s.mc.ListFile(ctx, req)
}

func (s *OssService) DeleteOssFile(ctx context.Context, req *fileV1.DeleteOssFileRequest) (*fileV1.DeleteOssFileResponse, error) {
	return s.mc.DeleteFile(ctx, req)
}

func (s *OssService) GetDownloadUrl(ctx context.Context, req *fileV1.GetDownloadInfoRequest) (*fileV1.GetDownloadInfoResponse, error) {
	return s.mc.GetDownloadUrl(ctx, req)
}
