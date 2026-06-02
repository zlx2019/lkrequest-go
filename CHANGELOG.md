# Changelog

本项目的所有重要变更都会记录在此文件中。

每次发布 `public-v<version>` tag 前,必须先在此文件**顶部**新增对应版本章节,
格式必须严格匹配 `## lkrequest-go <version>`(大小写、空格敏感),否则 CI
`github-sync` 会拒绝发布。

## lkrequest-go 0.1.0
### 🚀 Features
- Initial public release with Go bindings for lkrequest-ffi
- Linux x86_64 and Windows x86_64 prebuilt FFI libraries
- cgo and purego dual binding strategies
