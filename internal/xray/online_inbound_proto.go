package xray

import (
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// These additive GetUsersStats fields are proposed by the companion Xray PR.
// Read unknown fields as well as generated ones so the panel still builds with
// its upstream dependency pin. Stock/older cores report no capability; no forked
// Go module or unmerged dependency is required to build or use the panel.
func onlineInboundTracking(message protoreflect.Message) bool {
	if field := message.Descriptor().Fields().ByNumber(2); field != nil {
		return field.Kind() == protoreflect.BoolKind && !field.IsList() && message.Get(field).Bool()
	}
	var supported bool
	for _, value := range unknownOnlineFields(message.GetUnknown(), 2, protowire.VarintType) {
		n, _ := protowire.ConsumeVarint(value)
		supported = n != 0
	}
	return supported
}

func onlineInboundTags(message protoreflect.Message) []string {
	if field := message.Descriptor().Fields().ByNumber(4); field != nil {
		if field.Kind() != protoreflect.StringKind || !field.IsList() {
			return nil
		}
		list := message.Get(field).List()
		var tags []string
		for i := 0; i < list.Len(); i++ {
			tags = append(tags, list.Get(i).String())
		}
		return tags
	}
	var tags []string
	for _, value := range unknownOnlineFields(message.GetUnknown(), 4, protowire.BytesType) {
		tag, n := protowire.ConsumeString(value)
		if n < 0 || !utf8.ValidString(tag) {
			return nil
		}
		tags = append(tags, tag)
	}
	return tags
}

// An invalid/truncated extension yields no data, not a partial online result.
func unknownOnlineFields(data []byte, number protowire.Number, kind protowire.Type) [][]byte {
	var values [][]byte
	for len(data) > 0 {
		field, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			return nil
		}
		data = data[n:]
		size := protowire.ConsumeFieldValue(field, wireType, data)
		if size < 0 {
			return nil
		}
		if field == number && wireType == kind {
			values = append(values, data[:size])
		}
		data = data[size:]
	}
	return values
}
