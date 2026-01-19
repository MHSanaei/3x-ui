# 关于 `docker compose build` 无输出的说明

## 结论
- `docker-compose.yml` 里只有 `image:`，没有 `build:`，因此运行 `docker compose build` 基本不会有动作。
- `docker-compose.local.yml` 才包含 `build:` 配置。

## 正确用法
如果要本地构建：

```sh
docker compose -f docker-compose.local.yml build
```

如果要拉取镜像：

```sh
docker compose pull
```

## 可能的其他原因
- 在错误的目录执行，导致未读取到正确的 compose 文件。

## 可选的语法检查（不会构建）

```sh
docker compose -f docker-compose.local.yml config --quiet
```

## 本地构建后的启动方式

启动：

```sh
docker compose -f docker-compose.local.yml up -d
```

查看日志：

```sh
docker compose -f docker-compose.local.yml logs -f
```
