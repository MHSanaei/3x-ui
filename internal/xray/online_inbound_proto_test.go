package xray

import (
	"reflect"
	"testing"

	stats "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestOnlineInboundProtoLegacyAndExtendedCore(t *testing.T) {
	userWire := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), "alice")
	for _, tag := range []string{"vless", "hysteria"} {
		userWire = protowire.AppendString(protowire.AppendTag(userWire, 4, protowire.BytesType), tag)
	}
	responseWire := protowire.AppendVarint(protowire.AppendTag(nil, 2, protowire.VarintType), 1)
	user, response := &stats.UserStat{}, &stats.GetUsersStatsResponse{}
	if err := proto.Unmarshal(userWire, user); err != nil {
		t.Fatal(err)
	}
	if err := proto.Unmarshal(responseWire, response); err != nil {
		t.Fatal(err)
	}
	if user.GetEmail() != "alice" || !onlineInboundTracking(response.ProtoReflect()) || !reflect.DeepEqual(onlineInboundTags(user.ProtoReflect()), []string{"vless", "hysteria"}) {
		t.Fatal("additive fields did not survive decoding with the pinned core descriptors")
	}
	if onlineInboundTracking((&stats.GetUsersStatsResponse{}).ProtoReflect()) || onlineInboundTags((&stats.UserStat{}).ProtoReflect()) != nil {
		t.Fatal("stock core must remain unsupported, not an empty supported result")
	}

	// Also cover descriptors that already know the fields, as after a future
	// dependency update; unknown-only decoding would silently stop working then.
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Syntax: proto.String("proto3"), Name: proto.String("online-inbound-test.proto"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Metadata"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("inbound_tracking"), Number: proto.Int32(2), Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()},
				{Name: proto.String("inbound_tags"), Number: proto.Int32(4), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()},
			},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	known := dynamicpb.NewMessage(file.Messages().Get(0))
	if err := proto.Unmarshal(append(responseWire, userWire...), known); err != nil {
		t.Fatal(err)
	}
	if !onlineInboundTracking(known) || !reflect.DeepEqual(onlineInboundTags(known), []string{"vless", "hysteria"}) {
		t.Fatal("known descriptors must decode the same metadata")
	}
	known.Set(known.Descriptor().Fields().ByNumber(2), protoreflect.ValueOfBool(false))
	if onlineInboundTracking(known) {
		t.Fatal("explicit false capability ignored")
	}
}

func TestOnlineInboundProtoMalformedAndDuplicateFields(t *testing.T) {
	boolWire := func(value uint64) []byte {
		return protowire.AppendVarint(protowire.AppendTag(nil, 2, protowire.VarintType), value)
	}
	for _, tc := range []struct {
		name string
		wire []byte
		want bool
	}{
		{"last false wins", append(boolWire(1), boolWire(0)...), false},
		{"last true wins", append(boolWire(0), boolWire(1)...), true},
		{"malformed after valid", append(boolWire(1), 0x80), false},
		{"truncated value", []byte{0x10, 0x80}, false},
		{"wrong wire type", []byte{0x12, 0}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A dynamic empty descriptor keeps every field unknown even if the
			// test is built in a workspace with the companion Xray changes.
			file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
				Syntax: proto.String("proto3"), Name: proto.String("empty-online-test.proto"),
				MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("Empty")}},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			message := dynamicpb.NewMessage(file.Messages().Get(0))
			message.SetUnknown(tc.wire)
			if got := onlineInboundTracking(message); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			message.SetUnknown([]byte{0x22, 0x02, 0xff, 0xff})
			if onlineInboundTags(message) != nil {
				t.Fatal("invalid UTF-8 must not become a profile tag")
			}
		})
	}
}
