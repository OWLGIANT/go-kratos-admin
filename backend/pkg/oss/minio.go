package oss

import (
	"bytes"
	"context"
	"mime"
	"net/url"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/go-utils/trans"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	ossMinio "github.com/tx7do/kratos-bootstrap/oss/minio"

	fileV1 "go-wind-admin/api/gen/go/file/service/v1"
)

const (
	defaultExpiryTime = time.Minute * 60 // 1小时
)

type MinIOClient struct {
	mc   *minio.Client
	conf *conf.OSS
	log  *log.Helper
}

func NewMinIoClient(cfg *conf.Bootstrap, logger log.Logger) *MinIOClient {
	l := log.NewHelper(log.With(logger, "module", "minio/data/admin-service"))
	return &MinIOClient{
		log:  l,
		conf: cfg.Oss,
		mc:   ossMinio.NewClient(cfg.Oss),
	}
}

// ContentTypeToBucketName 根据文件类型获取bucket名称
func (c *MinIOClient) ContentTypeToBucketName(contentType string) string {
	h := strings.Split(contentType, "/")
	if len(h) != 2 {
		return "images"
	}

	var bucketName string
	switch h[0] {
	case "image":
		bucketName = "images"
		break
	case "video":
		bucketName = "videos"
		break
	case "audio":
		bucketName = "audios"
		break
	case "application", "text":
		bucketName = "docs"
		break
	default:
		bucketName = "files"
		break
	}

	return bucketName
}

// contentTypeToFileSuffix 根据文件类型获取文件后缀
func (c *MinIOClient) contentTypeToFileSuffix(contentType string) string {
	fileSuffix := ""

	switch contentType {
	case "text/plain":
		fileSuffix = ".txt"
		break

	case "image/jpeg":
		fileSuffix = ".jpg"
		break

	case "image/png":
		fileSuffix = ".png"
		break

	default:
		extensions, _ := mime.ExtensionsByType(contentType)
		if len(extensions) > 0 {
			fileSuffix = extensions[0]
		}
	}

	return fileSuffix
}

// GetClient returns the underlying MinIO client
func (c *MinIOClient) GetClient() *minio.Client {
	return c.mc
}

// JointObjectName Spliced objectName, containing hash-based folder structure (exported version)
func (c *MinIOClient) JointObjectName(contentType string, filePath, fileName *string) (string, string) {
	return c.jointObjectName(contentType, filePath, fileName)
}

// EnsureBucketExists Ensure that the specified bucket exists
func (c *MinIOClient) EnsureBucketExists(ctx context.Context, bucketName string) error {
	exists, err := c.mc.BucketExists(ctx, bucketName)
	if err != nil {
		return fileV1.ErrorInternalServerError("failed to check bucket existence: %s", bucketName)
	}

	if !exists {
		err = c.mc.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fileV1.ErrorInternalServerError("failed to create bucket: %s", bucketName)
		}
		c.log.Infof("Created bucket: %s", bucketName)
	}

	return nil
}

// jointObjectName 拼接objectName
func (c *MinIOClient) jointObjectName(contentType string, filePath, fileName *string) (string, string) {
	fileSuffix := c.contentTypeToFileSuffix(contentType)

	var _fileName string
	if fileName == nil {
		_fileName = uuid.New().String() + fileSuffix
	} else {
		_fileName = *fileName
	}

	var objectName string
	if filePath != nil {
		objectName = *filePath + "/" + _fileName
	} else {
		objectName = _fileName
	}

	return objectName, _fileName
}

// GetUploadPresignedUrl 获取上传地址
func (c *MinIOClient) GetUploadPresignedUrl(ctx context.Context, req *fileV1.GetUploadPresignedUrlRequest) (*fileV1.GetUploadPresignedUrlResponse, error) {
	var bucketName string
	if req.BucketName != nil {
		bucketName = req.GetBucketName()
	} else {
		bucketName = c.ContentTypeToBucketName(req.GetContentType())
	}

	objectName, _ := c.jointObjectName(req.GetContentType(), req.FilePath, req.FileName)

	expiry := defaultExpiryTime

	var uploadUrl string
	var downloadUrl string
	var formData map[string]string

	var err error
	var presignedURL *url.URL

	switch req.GetMethod() {
	case fileV1.GetUploadPresignedUrlRequest_Put:
		presignedURL, err = c.mc.PresignedPutObject(ctx, bucketName, objectName, expiry)
		if err != nil {
			c.log.Errorf("Failed to generate presigned PUT policy: %v", err)
			return nil, fileV1.ErrorUploadFailed("failed to generate presigned PUT policy")
		}

		uploadUrl = presignedURL.String()
		uploadUrl = c.replaceEndpointHost(downloadUrl, c.conf.Minio.UploadHost)

		downloadUrl = presignedURL.Host + "/" + bucketName + "/" + objectName
		downloadUrl = c.replaceEndpointHost(downloadUrl, c.conf.Minio.DownloadHost)
		if !strings.HasPrefix(downloadUrl, presignedURL.Scheme) {
			downloadUrl = presignedURL.Scheme + "://" + downloadUrl
		}

	case fileV1.GetUploadPresignedUrlRequest_Post:
		policy := minio.NewPostPolicy()
		_ = policy.SetBucket(bucketName)
		_ = policy.SetKey(objectName)
		_ = policy.SetExpires(time.Now().UTC().Add(expiry))
		_ = policy.SetContentType(req.GetContentType())

		presignedURL, formData, err = c.mc.PresignedPostPolicy(ctx, policy)
		if err != nil {
			c.log.Errorf("Failed to generate presigned POST policy: %v", err)
			return nil, fileV1.ErrorUploadFailed("failed to generate presigned POST policy")
		}

		uploadUrl = presignedURL.String()
		uploadUrl = c.replaceEndpointHost(downloadUrl, c.conf.Minio.UploadHost)

		downloadUrl = presignedURL.Host + "/" + bucketName + "/" + objectName
		uploadUrl = c.replaceEndpointHost(downloadUrl, c.conf.Minio.DownloadHost)
		if !strings.HasPrefix(downloadUrl, presignedURL.Scheme) {
			downloadUrl = presignedURL.Scheme + "://" + downloadUrl
		}
	}

	return &fileV1.GetUploadPresignedUrlResponse{
		UploadUrl:   uploadUrl,
		DownloadUrl: downloadUrl,
		ObjectName:  objectName,
		BucketName:  trans.Ptr(bucketName),
		FormData:    formData,
	}, nil
}

// ListFile 获取文件夹下面的文件列表
func (c *MinIOClient) ListFile(ctx context.Context, req *fileV1.ListOssFileRequest) (*fileV1.ListOssFileResponse, error) {
	resp := &fileV1.ListOssFileResponse{
		Files: make([]string, 0),
	}
	for object := range c.mc.ListObjects(ctx,
		req.GetBucketName(),
		minio.ListObjectsOptions{
			Prefix:    req.GetFolder(),
			Recursive: req.GetRecursive(),
		},
	) {
		//fmt.Printf("%+v\n", object)
		resp.Files = append(resp.Files, object.Key)
	}
	return resp, nil
}

// ListFileForUEditor 获取文件夹下面的文件列表
func (c *MinIOClient) ListFileForUEditor(ctx context.Context, bucketName string, folder string) (*fileV1.UEditorResponse, error) {
	resp := &fileV1.UEditorResponse{
		State: trans.Ptr("SUCCESS"),
		List:  make([]*fileV1.UEditorResponse_Item, 0),
	}
	for object := range c.mc.ListObjects(ctx,
		bucketName,
		minio.ListObjectsOptions{
			Prefix:    folder,
			Recursive: true,
		},
	) {
		//fmt.Printf("%+v\n", object)
		resp.List = append(resp.List, &fileV1.UEditorResponse_Item{
			Url:   "/" + bucketName + "/" + folder + object.Key,
			Mtime: object.LastModified.Unix(),
		})
	}

	resp.Start = trans.Ptr(int32(0))
	resp.Total = trans.Ptr(int32(len(resp.List)))

	return resp, nil
}

// DeleteFile 删除一个文件
func (c *MinIOClient) DeleteFile(ctx context.Context, req *fileV1.DeleteOssFileRequest) (*fileV1.DeleteOssFileResponse, error) {
	err := c.mc.RemoveObject(ctx, req.GetBucketName(), req.GetObjectName(), minio.RemoveObjectOptions{})
	if err != nil {
		c.log.Errorf("Failed to delete file: %v", err)
		return nil, fileV1.ErrorDeleteFailed("failed to delete file")
	}

	return &fileV1.DeleteOssFileResponse{}, nil
}

// UploadFile 上传文件
func (c *MinIOClient) UploadFile(ctx context.Context, bucketName string, objectName string, file []byte) (string, error) {
	reader := bytes.NewReader(file)

	_, err := c.mc.PutObject(
		ctx,
		bucketName,
		objectName,
		reader, reader.Size(),
		minio.PutObjectOptions{},
	)
	if err != nil {
		c.log.Errorf("Failed to upload file: %v", err)
		return "", fileV1.ErrorUploadFailed("failed to upload file")
	}

	downloadUrl := "/" + bucketName + "/" + objectName

	return downloadUrl, nil
}

// GetDownloadUrl 获取下载地址
func (c *MinIOClient) GetDownloadUrl(ctx context.Context, req *fileV1.GetDownloadInfoRequest) (*fileV1.GetDownloadInfoResponse, error) {
	if req.GetPreferPresignedUrl() {
		expires := defaultExpiryTime
		if req.PresignExpireSeconds != nil {
			expires = time.Second * time.Duration(req.GetPresignExpireSeconds())
		}
		presignedURL, err := c.mc.PresignedGetObject(
			ctx,
			req.GetStorageObject().GetBucketName(),
			req.GetStorageObject().GetObjectName(),
			expires,
			nil,
		)
		if err != nil {
			c.log.Errorf("Failed to generate presigned URL: %v", err)
			return nil, fileV1.ErrorDownloadFailed("failed to generate presigned URL")
		}

		downloadUrl := presignedURL.String()
		downloadUrl = c.replaceEndpointHost(downloadUrl, c.conf.Minio.DownloadHost)
		if !strings.HasPrefix(downloadUrl, presignedURL.Scheme) {
			downloadUrl = presignedURL.Scheme + "://" + downloadUrl
		}

		return &fileV1.GetDownloadInfoResponse{
			Content: &fileV1.GetDownloadInfoResponse_DownloadUrl{
				DownloadUrl: downloadUrl,
			},
		}, nil
	} else {
		opts := minio.GetObjectOptions{}

		c.setDownloadRange(&opts, req.RangeStart, req.RangeEnd)

		object, err := c.mc.GetObject(
			ctx,
			req.GetStorageObject().GetBucketName(),
			req.GetStorageObject().GetObjectName(),
			opts,
		)
		if err != nil {
			c.log.Errorf("Failed to get object: %v", err)
			return nil, fileV1.ErrorDownloadFailed("failed to get object")
		}

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(object)
		if err != nil {
			c.log.Errorf("Failed to read object: %v", err)
			return nil, fileV1.ErrorDownloadFailed("failed to read object")
		}

		resp := &fileV1.GetDownloadInfoResponse{
			Content: &fileV1.GetDownloadInfoResponse_File{
				File: buf.Bytes(),
			},
		}

		st, err := object.Stat()
		if err != nil {
			c.log.Errorf("Failed to stat object: %v", err)
			return nil, fileV1.ErrorDownloadFailed("failed to stat object")
		}

		if req.GetAcceptMime() != "" {
			resp.Mime = req.GetAcceptMime()
		} else {
			resp.Mime = st.ContentType
		}
		if resp.GetMime() == "" {
			resp.Mime = "application/octet-stream"
		}

		resp.Checksum = st.ChecksumSHA256
		resp.SourceFileName = st.Key
		resp.Size = st.Size
		resp.UpdatedAt = timeutil.TimeToTimestamppb(&st.LastModified)

		return resp, nil
	}
}

// DownloadFile 下载文件
func (c *MinIOClient) DownloadFile(ctx context.Context, req *fileV1.DownloadFileRequest) (*fileV1.DownloadFileResponse, error) {
	if req.GetPreferPresignedUrl() {
		expires := defaultExpiryTime
		if req.PresignExpireSeconds != nil {
			expires = time.Second * time.Duration(req.GetPresignExpireSeconds())
		}
		presignedURL, err := c.mc.PresignedGetObject(
			ctx,
			req.GetStorageObject().GetBucketName(),
			req.GetStorageObject().GetObjectName(),
			expires,
			nil,
		)
		if err != nil {
			c.log.Errorf("Failed to generate presigned URL: %v", err)
			return nil, fileV1.ErrorDownloadFailed("failed to generate presigned URL")
		}

		downloadUrl := presignedURL.String()
		downloadUrl = c.replaceEndpointHost(downloadUrl, c.conf.Minio.DownloadHost)
		if !strings.HasPrefix(downloadUrl, presignedURL.Scheme) {
			downloadUrl = presignedURL.Scheme + "://" + downloadUrl
		}

		return &fileV1.DownloadFileResponse{
			Content: &fileV1.DownloadFileResponse_DownloadUrl{
				DownloadUrl: downloadUrl,
			},
		}, nil
	} else {
		opts := minio.GetObjectOptions{}

		c.setDownloadRange(&opts, req.RangeStart, req.RangeEnd)

		object, err := c.mc.GetObject(
			ctx,
			req.GetStorageObject().GetBucketName(),
			req.GetStorageObject().GetObjectName(),
			opts,
		)
		if err != nil {
			c.log.Errorf("Failed to get object: %v", err)
			return nil, fileV1.ErrorDownloadFailed("failed to get object")
		}

		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(object)
		if err != nil {
			c.log.Errorf("Failed to read object: %v", err)
			return nil, fileV1.ErrorDownloadFailed("failed to read object")
		}

		resp := &fileV1.DownloadFileResponse{
			Content: &fileV1.DownloadFileResponse_File{
				File: buf.Bytes(),
			},
		}

		st, err := object.Stat()
		if err != nil {
			c.log.Errorf("Failed to stat object: %v", err)
			return nil, fileV1.ErrorDownloadFailed("failed to stat object")
		}

		if req.GetAcceptMime() != "" {
			resp.Mime = req.GetAcceptMime()
		} else {
			resp.Mime = st.ContentType
		}
		if resp.GetMime() == "" {
			resp.Mime = "application/octet-stream"
		}

		resp.Checksum = st.ChecksumSHA256
		resp.SourceFileName = st.Key
		resp.Size = st.Size
		resp.UpdatedAt = timeutil.TimeToTimestamppb(&st.LastModified)

		return resp, nil
	}
}

// setDownloadRange 设置下载范围
func (c *MinIOClient) setDownloadRange(opts *minio.GetObjectOptions, start, end *int64) {
	if opts == nil {
		return
	}

	if start != nil && end != nil {
		_ = opts.SetRange(*start, *end)
	} else if start != nil {
		_ = opts.SetRange(*start, 0)
	} else if end != nil {
		_ = opts.SetRange(0, *end)
	}
}

func (c *MinIOClient) replaceEndpointHost(rawURL, host string) string {
	if rawURL == "" || host == "" {
		return rawURL
	}
	return strings.Replace(rawURL, c.conf.Minio.Endpoint, host, -1)
}
