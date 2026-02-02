##################################
# 使用预编译的二进制文件构建镜像
# 请先运行 ./build.sh 生成二进制文件
##################################

# 使用 Alpine 作为基础镜像，因为它非常轻量级
FROM alpine:3.18

# 安装必要的证书（如果应用程序需要进行 HTTPS 请求）
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 设置工作目录
WORKDIR /app

# 复制预编译的二进制文件
COPY go-wind-admin /app/bin/server

# 拷贝配置文件
COPY app/admin/service/configs/ /app/configs

# 创建一个名为 appuser 的非 root 用户
RUN adduser -D appuser && \
    chown -R appuser:appuser /app

# 切换到非特权用户
USER appuser:appuser

# 暴露服务端口，根据你的实际服务端口进行修改
EXPOSE 8080

# 设置容器启动时执行的命令
CMD ["/app/bin/server", "-c", "/app/configs"]
