// SPDX-Licence-Identifier: EUPL-1.2

package api

import (
	"reflect"
	"testing"
	"time"
)

// externalEvent mirrors hubMessage's exported field shape so the reflection
// bridge in callExternalHub/hubMessageValue must field-copy into it (the
// types are not assignable or directly convertible across package boundaries
// when wrapped in a foreign struct with the same layout but different name).
type externalEvent struct {
	Type      string
	Channel   string
	ProcessID string
	Data      any
	Timestamp time.Time
}

// fakeExternalHub records Broadcast and SendToChannel calls so the test can
// assert the reflection bridge invoked them with the expected field values.
type fakeExternalHub struct {
	broadcasts []externalEvent
	sends      []struct {
		channel string
		event   externalEvent
	}
}

func (f *fakeExternalHub) Broadcast(e externalEvent) {
	f.broadcasts = append(f.broadcasts, e)
}

func (f *fakeExternalHub) SendToChannel(channel string, e externalEvent) {
	f.sends = append(f.sends, struct {
		channel string
		event   externalEvent
	}{channel, e})
}

func TestWebsocketBridge_emitHubEvent_LocalHub_Good(t *testing.T) {
	h := newHub()
	// The local-hub branch returns early on the broadcastMessage success path.
	emitHubEvent(h, "process", map[string]string{"id": "p1"})
}

func TestWebsocketBridge_emitHubEvent_External_Good(t *testing.T) {
	fake := &fakeExternalHub{}
	emitHubEvent(fake, "process", map[string]string{"id": "p1"})

	requireLen(t, fake.broadcasts, 1)
	assertEqual(t, "event", fake.broadcasts[0].Type)
	assertEqual(t, "process", fake.broadcasts[0].Channel)

	requireLen(t, fake.sends, 1)
	assertEqual(t, "process", fake.sends[0].channel)
	assertEqual(t, "event", fake.sends[0].event.Type)
}

func TestWebsocketBridge_callExternalHub_Bad(t *testing.T) {
	fake := &fakeExternalHub{}
	// A method name that does not exist on the target is a no-op (no panic).
	callExternalHub(fake, "NoSuchMethod", "process", hubMessage{Type: typeEvent})
	assertLen(t, fake.broadcasts, 0)
	assertLen(t, fake.sends, 0)
}

func TestWebsocketBridge_callExternalHub_Ugly(t *testing.T) {
	// A target that is not a struct/pointer with methods must not panic; the
	// deferred recover guards reflective Call against bad inputs.
	callExternalHub(42, "Broadcast", "process", hubMessage{Type: typeEvent})
	callExternalHub(nil, "Broadcast", "process", hubMessage{Type: typeEvent})
}

// wrongAritybroadcastHub has Broadcast/SendToChannel with mismatched arities,
// which the bridge must reject without calling them.
type wrongArityHub struct{ called bool }

func (w *wrongArityHub) Broadcast()                                    { w.called = true }
func (w *wrongArityHub) SendToChannel(_ string, _ externalEvent, _ int) { w.called = true }

func TestWebsocketBridge_callExternalHub_WrongArity_Ugly(t *testing.T) {
	w := &wrongArityHub{}
	callExternalHub(w, "Broadcast", "process", hubMessage{Type: typeEvent})
	callExternalHub(w, "SendToChannel", "process", hubMessage{Type: typeEvent})
	assertFalse(t, w.called)
}

func TestWebsocketBridge_hubMessageValue_Assignable_Good(t *testing.T) {
	// When the target type is hubMessage itself, the value is returned as-is.
	target := reflect.TypeOf(hubMessage{})
	msg := hubMessage{Type: typeEvent, Channel: "c"}
	got, ok := hubMessageValue(target, msg)
	requireTrue(t, ok)
	assertEqual(t, msg, got.Interface().(hubMessage))
}

func TestWebsocketBridge_hubMessageValue_FieldCopy_Good(t *testing.T) {
	// A foreign struct target triggers the field-copy path.
	target := reflect.TypeOf(externalEvent{})
	now := time.Now()
	msg := hubMessage{
		Type:      typeEvent,
		Channel:   "process",
		ProcessID: "p1",
		Data:      map[string]int{"n": 1},
		Timestamp: now,
	}
	got, ok := hubMessageValue(target, msg)
	requireTrue(t, ok)
	ev := got.Interface().(externalEvent)
	assertEqual(t, "event", ev.Type)
	assertEqual(t, "process", ev.Channel)
	assertEqual(t, "p1", ev.ProcessID)
	assertNotNil(t, ev.Data)
	assertEqual(t, now, ev.Timestamp)
}

func TestWebsocketBridge_hubMessageValue_NonStruct_Bad(t *testing.T) {
	// A non-struct, non-convertible target is rejected.
	target := reflect.TypeOf("")
	_, ok := hubMessageValue(target, hubMessage{Type: typeEvent})
	assertFalse(t, ok)
}

func TestWebsocketBridge_setStringField_Ugly(t *testing.T) {
	// Setting an unknown field, an unsettable field, or a non-string field is
	// a silent no-op.
	type holder struct {
		Name string
		Age  int
	}
	v := reflect.New(reflect.TypeOf(holder{})).Elem()
	setStringField(v, "Missing", "x") // unknown field
	setStringField(v, "Age", "x")     // wrong kind
	setStringField(v, "Name", "ok")   // valid
	got := v.Interface().(holder)
	assertEqual(t, "ok", got.Name)
	assertEqual(t, 0, got.Age)
}

func TestWebsocketBridge_setAnyField_Ugly(t *testing.T) {
	type holder struct {
		Data any
		N    int
	}
	v := reflect.New(reflect.TypeOf(holder{})).Elem()
	setAnyField(v, "Missing", "x")            // unknown field
	setAnyField(v, "Data", nil)               // nil value is ignored
	setAnyField(v, "Data", map[string]int{})  // assignable to any
	setAnyField(v, "N", 5)                     // convertible into int
	got := v.Interface().(holder)
	assertNotNil(t, got.Data)
	assertEqual(t, 5, got.N)
}

func TestWebsocketBridge_setTimeField_Ugly(t *testing.T) {
	type holder struct {
		When time.Time
		Name string
	}
	now := time.Now()
	v := reflect.New(reflect.TypeOf(holder{})).Elem()
	setTimeField(v, "Missing", now) // unknown field
	setTimeField(v, "Name", now)    // not assignable from time.Time
	setTimeField(v, "When", now)    // valid
	got := v.Interface().(holder)
	assertEqual(t, now, got.When)
	assertEqual(t, "", got.Name)
}
