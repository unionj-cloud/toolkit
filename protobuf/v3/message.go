package v3

import (
	"encoding/json"
	"fmt"
	"github.com/gobuffalo/flect"
	"github.com/goccy/go-reflect"
	"github.com/lithammer/shortuuid/v4"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"unicode"

	"github.com/iancoleman/strcase"
	"github.com/unionj-cloud/toolkit/astutils"
	"github.com/unionj-cloud/toolkit/sliceutils"
	"github.com/unionj-cloud/toolkit/stringutils"
)

var re = regexp.MustCompile(`json:"(.*?)"`)

var _ ProtobufType = (*Enum)(nil)
var _ ProtobufType = (*Message)(nil)

type ProtobufType interface {
	GetName() string
	String() string
	Inner() bool
}

var MessageStore = make(map[string]Message)

var EnumStore = make(map[string]Enum)

var ImportStore = make(map[string]struct{})

var MessageNames []string

type EnumField struct {
	Name   string
	Number int
}

func (receiver ProtoGenerator) newEnumField(field string, index int) EnumField {
	return EnumField{
		Name:   strings.ToUpper(strcase.ToSnake(field)),
		Number: index,
	}
}

type Enum struct {
	Name   string
	Fields []EnumField
}

func (e Enum) Inner() bool {
	return false
}

func (e Enum) String() string {
	return e.Name
}

func (e Enum) GetName() string {
	return e.Name
}

func (receiver ProtoGenerator) NewEnum(enumMeta astutils.EnumMeta) Enum {
	var fields []EnumField
	for i, field := range enumMeta.Values {
		fields = append(fields, receiver.newEnumField(field, i))
	}
	return Enum{
		Name:   flect.Capitalize(enumMeta.Name),
		Fields: fields,
	}
}

// Message represents protobuf message definition
type Message struct {
	Name       string
	Fields     []Field
	Comments   []string
	IsInner    bool
	IsScalar   bool
	IsMap      bool
	IsRepeated bool
	IsTopLevel bool
	// IsImported denotes the message will be imported from third-party, such as from google/protobuf
	IsImported bool
}

func (m Message) Inner() bool {
	return m.IsInner
}

func (m Message) GetName() string {
	return m.Name
}

func (m Message) String() string {
	switch {
	case reflect.DeepEqual(m, Any):
		return "anypb.Any"
	case reflect.DeepEqual(m, Struct):
		return "structpb.Struct"
	case reflect.DeepEqual(m, Value):
		return "structpb.Value"
	case reflect.DeepEqual(m, ListValue):
		return "structpb.ListValue"
	case reflect.DeepEqual(m, Empty):
		return "emptypb.Empty"
	default:
		return m.Name
	}
}

// NewMessage returns message instance from astutils.StructMeta
func (receiver ProtoGenerator) NewMessage(structmeta astutils.StructMeta) Message {
	var fields []Field
	for i, field := range structmeta.Fields {
		fields = append(fields, receiver.newField(field, i+1))
	}
	return Message{
		Name:       flect.Capitalize(structmeta.Name),
		Fields:     fields,
		Comments:   structmeta.Comments,
		IsTopLevel: true,
	}
}

// Field represents protobuf message field definition
type Field struct {
	Name     string
	Type     ProtobufType
	Number   int
	Comments []string
	JsonName string
}

func (receiver ProtoGenerator) newField(field astutils.FieldMeta, index int) Field {
	t := receiver.MessageOf(field.Type)
	if t.Inner() {
		message := t.(Message)
		message.Name = flect.Capitalize(field.Name)
		t = message
	}
	var fieldName string
	if stringutils.IsNotEmpty(field.Tag) && re.MatchString(field.Tag) {
		jsonName := re.FindStringSubmatch(field.Tag)[1]
		fieldName = strings.Split(jsonName, ",")[0]
		if fieldName == "-" {
			fieldName = receiver.fieldNamingFunc(field.Name)
		}
	} else {
		fieldName = receiver.fieldNamingFunc(field.Name)
	}
	return Field{
		Name:     fieldName,
		Type:     t,
		Number:   index,
		Comments: field.Comments,
		JsonName: fieldName,
	}
}

var (
	Double = Message{
		Name:     "double",
		IsScalar: true,
	}
	Float = Message{
		Name:     "float",
		IsScalar: true,
	}
	Int32 = Message{
		Name:     "int32",
		IsScalar: true,
	}
	Int64 = Message{
		Name:     "int64",
		IsScalar: true,
	}
	Uint32 = Message{
		Name:     "uint32",
		IsScalar: true,
	}
	Uint64 = Message{
		Name:     "uint64",
		IsScalar: true,
	}
	Bool = Message{
		Name:     "bool",
		IsScalar: true,
	}
	String = Message{
		Name:     "string",
		IsScalar: true,
	}
	Bytes = Message{
		Name:     "bytes",
		IsScalar: true,
	}
	Any = Message{
		Name:       "google.protobuf.Any",
		IsTopLevel: true,
		IsImported: true,
	}
	Struct = Message{
		Name:       "google.protobuf.Struct",
		IsTopLevel: true,
		IsImported: true,
	}
	Value = Message{
		Name:       "google.protobuf.Value",
		IsTopLevel: true,
		IsImported: true,
	}
	ListValue = Message{
		Name:       "google.protobuf.ListValue",
		IsTopLevel: true,
		IsImported: true,
	}
	Empty = Message{
		Name:       "google.protobuf.Empty",
		IsTopLevel: true,
		IsImported: true,
	}
	Time = Message{
		Name:     "google.protobuf.Timestamp",
		IsScalar: true,
	}
)

func (receiver ProtoGenerator) MessageOf(ft string) ProtobufType {
	if astutils.IsVarargs(ft) {
		ft = astutils.ToSlice(ft)
	}
	ft = strings.TrimLeft(ft, "*")
	switch ft {
	case "int", "int8", "int16", "int32", "byte", "rune", "complex64", "complex128":
		return Int32
	case "uint", "uint8", "uint16", "uint32":
		return Uint32
	case "int64":
		return Int64
	case "uint64", "uintptr":
		return Uint64
	case "bool":
		return Bool
	case "string", "error", "[]rune", "decimal.Decimal":
		return String
	case "[]byte", "v3.FileModel", "os.File":
		return Bytes
	case "float32":
		return Float
	case "float64":
		return Double
	case "time.Time", "gorm.DeletedAt", "customtypes.Time":
		//ImportStore["google/protobuf/timestamp.proto"] = struct{}{}
		//return Time
		return String
	default:
		return receiver.handleDefaultCase(ft)
	}
}

var anonystructre *regexp.Regexp

func init() {
	anonystructre = regexp.MustCompile(`anonystruct«(.*)»`)
}

func (receiver ProtoGenerator) handleDefaultCase(ft string) ProtobufType {
	var title string
	if ft == "map[string]interface{}" {
		ImportStore["google/protobuf/struct.proto"] = struct{}{}
		return Struct
	}
	if ft == "[]interface{}" {
		ImportStore["google/protobuf/struct.proto"] = struct{}{}
		return ListValue
	}
	if strings.HasPrefix(ft, "map[") {
		elem := ft[strings.Index(ft, "]")+1:]
		key := ft[4:strings.Index(ft, "]")]
		keyMessage := receiver.MessageOf(key)
		if reflect.DeepEqual(keyMessage, Float) || reflect.DeepEqual(keyMessage, Double) || reflect.DeepEqual(keyMessage, Bytes) {
			log.Error("floating point types and bytes cannot be key_type of maps, please refer to https://developers.google.com/protocol-buffers/docs/proto3#maps")
			goto ANY
		}
		elemMessage := receiver.MessageOf(elem)
		if strings.HasPrefix(elemMessage.GetName(), "map<") {
			log.Error("the value_type cannot be another map, please refer to https://developers.google.com/protocol-buffers/docs/proto3#maps")
			goto ANY
		}
		return Message{
			Name:  fmt.Sprintf("map<%s, %s>", keyMessage.GetName(), elemMessage.GetName()),
			IsMap: true,
		}
	}
	if strings.HasPrefix(ft, "[") {
		elem := ft[strings.Index(ft, "]")+1:]
		elemMessage := receiver.MessageOf(elem)
		if strings.HasPrefix(elemMessage.GetName(), "map<") {
			log.Error("map fields cannot be repeated, please refer to https://developers.google.com/protocol-buffers/docs/proto3#maps")
			goto ANY
		}
		messageName := elemMessage.GetName()
		if strings.Contains(elemMessage.GetName(), "repeated ") {
			messageName = messageName[strings.LastIndex(messageName, ".")+1:]
			messageName = "Nested" + flect.Capitalize(messageName)
			fieldName := receiver.fieldNamingFunc(messageName)
			MessageStore[messageName] = Message{
				Name: messageName,
				Fields: []Field{
					{
						Name:     fieldName,
						Type:     elemMessage,
						Number:   1,
						JsonName: fieldName,
					},
				},
				IsInner: true,
			}
		}
		return Message{
			Name:       fmt.Sprintf("repeated %s", messageName),
			IsRepeated: true,
		}
	}
	if anonystructre.MatchString(ft) {
		result := anonystructre.FindStringSubmatch(ft)
		var structmeta astutils.StructMeta
		json.Unmarshal([]byte(result[1]), &structmeta)
		message := receiver.NewMessage(structmeta)
		message.IsInner = true
		message.IsTopLevel = false
		message.Name = "Anonystruct" + shortuuid.NewWithNamespace(result[1])
		MessageStore[message.Name] = message
		return message
	}
	if !strings.Contains(ft, ".") {
		title = ft
	}
	if stringutils.IsEmpty(title) {
		title = ft[strings.LastIndex(ft, ".")+1:]
	}
	if stringutils.IsNotEmpty(title) {
		if unicode.IsUpper(rune(title[0])) {
			if sliceutils.StringContains(MessageNames, title) {
				return Message{
					Name:       flect.Capitalize(title),
					IsTopLevel: true,
				}
			}
		}
		if e, ok := EnumStore[title]; ok {
			return e
		}
	}
ANY:
	ImportStore["google/protobuf/struct.proto"] = struct{}{}
	return Value
}
