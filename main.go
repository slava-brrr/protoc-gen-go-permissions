package main

import (
	_ "embed"
	"flag"
	"fmt"
	"google.golang.org/protobuf/encoding/protowire"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	//go:embed tmpl/permissions.go.tmpl
	permissionsFuncTmpl string

	//go:embed tmpl/context_interceptor.go.tmpl
	contextInterceptorTmpl string

	//go:embed tmpl/header.go.tmpl
	headerTmpl string
)

var (
	// extensionFieldNumber - номер поля расширения для required_permissions
	extensionFieldNumber = flag.Int("extension-field-number", 50001, "Field number for the required_permissions extension")
)

func main() {
	var flags flag.FlagSet
	flags.IntVar(extensionFieldNumber, "extension-field-number", 50001, "Field number for the required_permissions extension")

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if err := generateFile(gen, f); err != nil {
				return err
			}
		}
		return nil
	})
}

func generateFile(gen *protogen.Plugin, file *protogen.File) error {
	if len(file.Services) == 0 {
		return nil
	}

	filename := file.GeneratedFilenamePrefix + "_permissions.pb.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)

	g.P(fmt.Sprintf(headerTmpl, file.Desc.Path(), file.GoPackageName))

	// --- Permissions ---
	//g.P("var required_permissions = map[string][]string {")

	var mapData strings.Builder

	for _, service := range file.Services {
		for _, method := range service.Methods {
			// 1. Получаем полное имя метода (ключ карты)
			fullMethodName := fmt.Sprintf("/%s.%s/%s",
				file.Desc.Package(), service.Desc.Name(), method.Desc.Name())

			// 2. Получаем дескриптор опций метода
			opts := method.Desc.Options()

			// 3. Проверяем наличие нашего расширения required_permissions
			// Используем рефлексию для получения неизвестных полей опций
			requiredPermsStr := extractUnknownStrings(opts, *extensionFieldNumber)

			// 4. Парсим полученную строку в слайс строк
			if len(requiredPermsStr) == 0 {
				// Опция не установлена или пуста, пропускаем метод.
				continue
			}
			requiredPerms := strings.Split(requiredPermsStr, ",")

			// 5. Записываем в карту
			mapData.WriteString(fmt.Sprintf("\n\t%q: {\n", fullMethodName))
			for _, perm := range requiredPerms {
				mapData.WriteString(fmt.Sprintf("\t\t%q,\n", perm))
			}
			mapData.WriteString("\t\n},")
		}
	}

	g.P(fmt.Sprintf(permissionsFuncTmpl, mapData.String()))
	g.P(contextInterceptorTmpl)

	return nil
}

func extractUnknownStrings(opts protoreflect.ProtoMessage, fieldNumber int) string {
	raw := opts.ProtoReflect().GetUnknown()

	for len(raw) > 0 {
		num, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			break
		}
		raw = raw[n:]
		if num != protowire.Number(fieldNumber) {
			// не то поле, пропускаем
			skip := protowire.ConsumeFieldValue(num, typ, raw)
			if skip < 0 {
				break
			}
			raw = raw[skip:]
			continue
		}

		// читаем строку (length-delimited)
		val, m := protowire.ConsumeString(raw)
		if m < 0 {
			break
		}
		return val
	}
	return ""
}
