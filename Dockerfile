# 多阶段构建:插件化框架应用(api/<name> 或根 main.go)。
# 构建参数 APP_PATH 指定入口目录,默认仓库根 main.go。
#
# 示例:
#   docker build -t ggr:latest .
#   docker build --build-arg APP_PATH=./api/demo-app -t demo-app:latest .
FROM golang:1.25-alpine AS builder

ARG APP_PATH=.
WORKDIR /src

# 先拷贝模块定义,利用层缓存加速依赖下载
COPY go.mod go.sum ./
COPY go.work go.work.sum ./
COPY plugins/*/go.mod plugins/*/
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ${APP_PATH}

FROM alpine:3.20

RUN adduser -D -u 10001 appuser && apk add --no-cache ca-certificates tzdata
USER appuser

COPY --from=builder /out/app /usr/local/bin/app

# 配置与静态资源按需挂载:/app/configs
WORKDIR /app
EXPOSE 8080

ENTRYPOINT ["app"]
CMD ["./configs/config.yaml"]
