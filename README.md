## protoc-gen-go-permissions

Минимальный плагин для `protoc`, который генерирует:
- карту требуемых прав для каждого gRPC-метода;
- unary‑интерсептор сервера, добавляющий эти права в `context` запроса.

Плагин читает строковую метод‑опцию (список прав через запятую) по её числовому полю и генерирует файл вида `*_permissions.pb.go` в тот же Go‑пакет, что и код сервиса.

### Установка / Сборка

- Если модуль доступен:
  ```bash
  go install gitlab.wildberries.ru/unified-logistics/shared/protoc-gen-go-permissions@latest
  ```
- Из исходников (из этого каталога):
  ```bash
  go build -o protoc-gen-go-permissions .
  # Добавьте бинарник в PATH, чтобы protoc смог его найти
  ```

### Генерация через protoc

Базовая генерация (добавьте к обычным `--go_out`/`--go-grpc_out`):
```bash
protoc -I . \
  --go_out=. --go-grpc_out=. \
  --go-permissions_out=. \
  path/to/your/service.proto
```

Настройка номерa поля опции (по умолчанию `50001`):
```bash
protoc -I . \
  --go-permissions_out=extension-field-number=50042:. \
  path/to/your/service.proto
```

Примечания:
- Плагин ищет строковую метод‑опцию с указанным числовым полем и делит её по запятым.
- В ваших `.proto` необходимо объявить расширение `google.protobuf.MethodOptions` (например, `(required_permissions)`), использующее этот номер поля, и подключить его там, где вы ставите опцию.

### Определение опции в .proto (пример)

```proto
syntax = "proto3";
package calls;

import "google/protobuf/descriptor.proto";
// В отдельном файле объявите расширение, например:
// extend google.protobuf.MethodOptions { string required_permissions = 50001; }

service PhoneCalls {
  rpc GetCall (GetCallRequest) returns (GetCallResponse) {
    option (required_permissions) = "object_1:action_1,object_n:action_m";
  }
  rpc UpdateCall (UpdateCallRequest) returns (UpdateCallResponse) {
    option (required_permissions) = "object:action";
  }
}
```

### Примеры использования в Go

---
##### С помощью Interceptor

После генерации в том же Go‑пакете будут доступны функции: `UnaryPermissionsInterceptor()`, `PermissionsFromContext(ctx)` и `Permissions()`.

1) Регистрация интерсептора на gRPC‑сервере
```go
s := grpc.NewServer(
    grpc.UnaryInterceptor(calls.UnaryPermissionsInterceptor()),
)
// где `calls` — пакет сгенерированного кода сервиса/прав
```

2) Получение прав в хендлере
```go
func (s *Server) GetCall(ctx context.Context, req *calls.GetCallRequest) (*calls.GetCallResponse, error) {
    perms := calls.PermissionsFromContext(ctx)
    // авторизация на основании perms
    // ...
    return &calls.GetCallResponse{}, nil
}
```
---
##### Прямое получение пермишенов
```go
perms := calls.PermissionsByFullMethod(info.FullMethod) // map["/calls.PhoneCalls/GetCall"] = []string{"calls.read","calls.view_sensitive"}
```
---
###### Дополнительно: просмотр всей карты соответствий во время выполнения
```go
permMap := calls.Permissions() // map[FullMethod][]Permission
```

### Как это работает

Для каждого метода сервисов генератор читает указанную метод‑опцию (по номеру поля), разбивает строку по запятым и строит `map[string][]string`, где ключ — полный путь gRPC‑метода (`/package.Service/Method`). Интерсептор помещает найденный список прав в контекст запроса для дальнейшего использования в обработчиках.

