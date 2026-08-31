# objectstore

与业务无关的对象存储封装，供配图转存、用户头像等复用。

## 用法

```go
store := objectstore.New(objectstore.Options{ /* R2 配置 */ })
// 未配齐时 store == nil，调用前判断即可，无需空实现。

if store != nil {
    url, err := objectstore.PutBytes(ctx, store, "avatars", "image/png", pngBytes)
    // ...
}

// 配图转存：nil 时 PublishSource 原样返回 source
url, err := objectstore.PublishSource(ctx, store, sourceURL, "pexels", 0)
```

## 分层

- **pkg/objectstore**：接口 + R2 实现 + URL/data 辅助
- **config.R2**：应用配置（yml/env）
- **infra/image**：`if g.store != nil` 再转存
- 未来头像：同一 `Store`，`folder="avatars"`
