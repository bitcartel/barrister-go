package barrister

import (
	. "github.com/sdegutis/go.assert"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"
)

var strField = &Field{Type: "string", Optional: false, IsArray: false}
var enumField = &Field{Type: "StringAlias", Optional: false, IsArray: false}
var arrField = &Field{Type: "float", Optional: false, IsArray: true}

var noNestStruct = &Struct{Name: "NoNesting", Fields: []Field{
		Field{Name: "a", Type: "string", Optional: true, IsArray: false},
		Field{Name: "b", Type: "int", Optional: true, IsArray: false},
		Field{Name: "C", Type: "float", Optional: true, IsArray: false},
		Field{Name: "d", Type: "bool", Optional: true, IsArray: false},
		Field{Name: "E", Type: "string", Optional: true, IsArray: true},
	}}
var noNestField = &Field{Type: "NoNesting", Optional: false, IsArray: true}

var nestStruct = &Struct{Name: "Nested", Fields: []Field{
		Field{Name: "name", Type: "string", Optional: false, IsArray: false},
		Field{Name: "Nest", Type: "NoNesting", Optional: false, IsArray: false},
	}}
var nestField = &Field{Type: "Nested", Optional: false, IsArray: true}

type ConvertTest struct {
	target interface{}
	input  interface{}
	field  *Field
	ok     bool
}

type NoNesting struct {
	A string
	B int64
	C float64
	D bool
	E []string
}

type StringAlias string

type Nested struct {
	Name string
	Nest NoNesting
}

type BImpl struct{}

func (b BImpl) Echo(s string) (*string, *JsonRpcError) {
	if s == "return-null" {
		return nil, nil
	}
	return &s, nil
}

type BImpl_MissingFunc struct {}

type BImpl_BadParam struct {}
func (b BImpl_BadParam) Echo(f float64) (*string, *JsonRpcError) {
	s := "blah"
	return &s, nil
}

type BImpl_BadReturn struct {}
func (b BImpl_BadReturn) Echo(s string) (int, *JsonRpcError) {
	return 10, nil
}

type BImpl_BadReturn2 struct {}
func (b BImpl_BadReturn2) Echo(s string) (*string, int) {
	s2 := "blah"
	return &s2, 0
}

type BImpl_BadReturn3 struct {}
func (b BImpl_BadReturn3) Echo(s string) (*string) {
	s2 := "blah"
	return &s2
}

type EchoCall struct {
	in  string
	out interface{}
}

type ValidateCase struct {
	field Field
	val   interface{}
	err   string
}

type CallFail struct {
	method  string
	errcode int
}

//////////////////////////////////////

func readFile(fname string) []byte {
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		panic(err)
	}
	return b
}

func readConformJson() []byte {
	return readFile("test/conform.json")
}

func parseTestIdl() *Idl {
	idl, err := ParseIdlJson(readConformJson())
	if err != nil {
		panic(err)
	}
	return idl
}

func createTestIdl() *Idl {
	idl := &Idl{structs: map[string]*Struct{}, enums: map[string][]EnumValue{}}
	idl.structs["NoNesting"] = noNestStruct
	idl.structs["Nested"] = nestStruct
	idl.enums["StringAlias"] = []EnumValue{
		EnumValue{"blah", ""},
		EnumValue{"foo", ""},
	}
	idl.computeAllStructFields()
	return idl
}

// not ready yet..
func testIdl2Go(t *testing.T) {
	idl := parseTestIdl()

	code := idl.GenerateGo("conform")
	ioutil.WriteFile("conform.go", code, 0644)
}

func TestParseIdlJson(t *testing.T) {
	idl := parseTestIdl()

	meta := Meta{BarristerVersion: "0.1.2", DateGenerated: 1337654725230000000, Checksum: "34f6238ed03c6319017382e0fdc638a7"}

	expected := Idl{Meta: meta}
	expected.elems = append(expected.elems, IdlJsonElem{Type: "comment", Value: "Barrister conformance IDL\n\nThe bits in here have silly names and the operations\nare not intended to be useful.  The intent is to\nexercise as much of the IDL grammar as possible"})

	enumVals := []EnumValue{
		EnumValue{Value: "ok"},
		EnumValue{Value: "err"},
	}
	expected.elems = append(expected.elems,
		IdlJsonElem{Type: "enum", Name: "Status", Values: enumVals})

	enumVals2 := []EnumValue{
		EnumValue{Value: "add"},
		EnumValue{Value: "multiply", Comment: "mult comment"},
	}
	expected.elems = append(expected.elems,
		IdlJsonElem{Type: "enum", Name: "MathOp", Values: enumVals2})

	fields := []Field{
		Field{Optional: false, IsArray: false, Type: "Status", Name: "status"},
	}
	expected.elems = append(expected.elems, IdlJsonElem{
		Type: "struct", Name: "Response", Fields: fields})

	fields2 := []Field{
		Field{Optional: false, IsArray: false, Type: "int", Name: "count"},
		Field{Optional: false, IsArray: true, Type: "string", Name: "items"},
	}
	expected.elems = append(expected.elems,
		IdlJsonElem{Type: "struct", Name: "RepeatResponse",
			Extends: "Response", Fields: fields2,
			Comment: "testing struct inheritance"})

	DeepEquals(t, expected.Meta, idl.Meta)
	Equals(t, len(idl.elems), 11)

	for i, ex := range expected.elems {
		DeepEquals(t, ex, idl.elems[i])
	}

	Equals(t, 2, len(idl.interfaces))
	Equals(t, 7, len(idl.interfaces["A"]))
	Equals(t, 1, len(idl.interfaces["B"]))

	methodKeys := []string{
		"A.add", "A.calc", "A.sqrt", "A.repeat", "A.say_hi",
		"A.repeat_num", "A.putPerson", "B.echo",
	}
	for _, key := range methodKeys {
		_, ok := idl.methods[key]
		if !ok {
			t.Errorf("No method with key: %s", key)
		}
	}

	structKeys := []string{
		"Response", "RepeatResponse", "HiResponse", "RepeatRequest", "Person",
	}
	for _, key := range structKeys {
		_, ok := idl.structs[key]
		if !ok {
			t.Errorf("No struct with key: %s", key)
		}
	}

	enumKeys := []string{
		"Status", "MathOp",
	}
	for _, key := range enumKeys {
		_, ok := idl.enums[key]
		if !ok {
			t.Errorf("No enum with key: %s", key)
		}
	}

	mathOps := []EnumValue{
		EnumValue{"add", ""},
		EnumValue{"multiply", "mult comment"},
	}
	if !reflect.DeepEqual(idl.enums["MathOp"], mathOps) {
		t.Errorf("MathOp enum: %v != %v", idl.enums["MathOp"], mathOps)
	}

}

func TestServerBarristerIdl(t *testing.T) {
	idl := parseTestIdl()
	svr := NewServer(idl)

	rpcReq := JsonRpcRequest{Id: "123", Method: "barrister-idl", Params: ""}
	reqJson, _ := json.Marshal(rpcReq)
	respJson := svr.InvokeJSON(reqJson)
	rpcResp := BarristerIdlRpcResponse{}
	err := json.Unmarshal(respJson, &rpcResp)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("%v\n", rpcResp.Result)

	DeepEquals(t, idl.elems, rpcResp.Result)
}

func TestServerCallSuccess(t *testing.T) {
	bimpl := BImpl{}
	idl := parseTestIdl()
	svr := NewServer(idl)
	svr.AddHandler("B", bimpl)

	calls := []EchoCall{
		EchoCall{"hi", "hi"},
		EchoCall{"2", "2"},
		EchoCall{"return-null", nil},
	}

	for _, call := range calls {
		res, err := svr.Call("B.echo", call.in)
		if err != nil {
			panic(err)
		}

		resStr, ok := res.(*string)
		if !ok {
			s := fmt.Sprintf("B.echo return val cannot be converted to *string. type=%v",
				reflect.TypeOf(res).Name())
			panic(s)
		}

		if !((resStr == nil && call.out == nil) || (*resStr == call.out)) {
			t.Errorf("B.echo %v != %v", resStr, call.out)
		}
	}
}

func TestServerCallFail(t *testing.T) {
	bimpl := BImpl{}
	idl := parseTestIdl()
	svr := NewServer(idl)
	svr.AddHandler("B", bimpl)

	calls := []CallFail{
		CallFail{"B.", -32601},
		CallFail{"", -32601},
		CallFail{"B.foo", -32601},
		CallFail{"B.echo", -32602},
	}

	for _, call := range calls {
		res, err := svr.Call(call.method)
		if res != nil {
			t.Errorf("%v != nil on expected fail call: %s", res, call.method)
		} else if err == nil {
			t.Errorf("err == nil on expected fail call: %s", call.method)
		} else if err.Code != call.errcode {
			t.Errorf("errcode %d != %d on expected fail call: %s", err.Code,
				call.errcode, call.method)
		}
	}
}

func TestParseMethod(t *testing.T) {
	cases := [][]string{
		[]string{"B.echo", "B", "Echo"},
		[]string{"B.", "B.", ""},
		[]string{"Cat.a", "Cat", "A"},
		[]string{"barrister-idl", "barrister-idl", ""},
	}

	for _, c := range cases {
		iface, fname := ParseMethod(c[0])
		Equals(t, iface, c[1])
		Equals(t, fname, c[2])
	}
}

func TestParseStuff(t *testing.T) {
	s := []byte(`{"jsonrpc":"2.0", "id":"123", "method": "blah", "params":["a","b"]}`)
	target := map[string]interface{}{}
	err := json.Unmarshal(s, &target)
	if err != nil {
		panic(err)
	}
}

func TestConvert(t *testing.T) {
	idl := createTestIdl()

	cases := []ConvertTest{
		ConvertTest{"hi", "hi", strField, true},
		ConvertTest{"", 10, strField, false},
		ConvertTest{[]float64{1, 2.1, 3}, []interface{}{1, 2.1, 3}, arrField, true},
		ConvertTest{StringAlias("blah"), "blah", enumField, true},
		ConvertTest{StringAlias("invalid"), "invalid", enumField, false},
		ConvertTest{NoNesting{A: "hi", B: 30}, map[string]interface{}{"a": "hi", "b": 30}, noNestField, true},
		ConvertTest{NoNesting{}, map[string]interface{}{"a": "hi", "b": "foo"}, noNestField, false},
		ConvertTest{NoNesting{C: 3.2, D: true}, map[string]interface{}{"C": 3.2, "d": true}, noNestField, true},
		ConvertTest{NoNesting{C: 2.8, D: false}, map[string]interface{}{"C": 2.8, "D": false}, noNestField, true},
		ConvertTest{NoNesting{E: []string{"a", "b"}}, map[string]interface{}{"E": []string{"a", "b"}}, noNestField, true},
		ConvertTest{Nested{Name: "hi", Nest: NoNesting{B: 30}}, map[string]interface{}{"name": "hi", "Nest": map[string]interface{}{"b": 30.0}}, nestField, true},
	}

	for x, test := range cases {
		msg := fmt.Sprintf("TestConvert[%d]", x)
		targetType := reflect.TypeOf(test.target)
		conv := NewConvert(idl, test.field, targetType, test.input, msg, false)
		val, err := conv.Run()
		if test.ok {
			if err != nil {
				t.Errorf("%s - Couldn't convert %v to %v: %v",
					msg, test.input, reflect.TypeOf(test.target), err)
			} else {
				if val.Kind() == reflect.Ptr {
					val = val.Elem()
				}

				if val.Type() != targetType {
					t.Errorf("%s - Return type: %v != %v", msg, val.Type(), targetType)
				} else if !reflect.DeepEqual(val.Interface(), test.target) {
					t.Errorf("%s - Expected %v but was %v", msg, test.target, val.Interface())
				}
			}
		} else if err == nil {
			t.Errorf("%s - Expected err converting %v to %v, but it worked: %v",
				msg, test.input, reflect.TypeOf(test.target).Name(), val.Interface())
		}
	}
}

func TestServerInvokeJSONSuccess(t *testing.T) {
	bimpl := BImpl{}
	idl := parseTestIdl()
	svr := NewServer(idl)
	svr.AddHandler("B", bimpl)

	calls := []EchoCall{
		EchoCall{"hi", "hi"},
		EchoCall{"2", "2"},
		EchoCall{"return-null", nil},
	}

	for _, call := range calls {
		req := JsonRpcRequest{Id: "123", Method: "B.echo", Params: []interface{}{call.in}}
		reqBytes, err := json.Marshal(req)
		if err != nil {
			panic(err)
		}

		resBytes := svr.InvokeJSON(reqBytes)
		resp := JsonRpcResponse{}
		err = json.Unmarshal(resBytes, &resp)
		if err != nil {
			panic(err)
		}

		if resp.Error != nil {
			t.Errorf("B.echo %v returned err: %v", call.in, resp.Error)
		} else {
			res := resp.Result
			if res == nil {
				if call.out != nil {
					t.Errorf("B.echo nil != %v", call.out)
				}
			} else {
				resStr, ok := res.(string)
				if !ok {
					n := reflect.TypeOf(res).Name()
					t.Errorf("B.echo return val cannot be converted to string. type=%v", n)
				}

				if resStr != call.out {
					t.Errorf("B.echo %v != %v", resStr, call.out)
				}
			}
		}

	}
}

func TestAddHandlerPanicsIfIfaceNotInIdl(t *testing.T) {
	idl := createTestIdl()
	svr := NewServer(idl)

	fx := func() {
		defer func() {
			if r := recover(); r != nil {
				// ok
			}
		}()
		svr.AddHandler("C", BImpl{})
		t.Errorf("AddHandler didn't panic when called w/invalid iface name")
	}
	fx()
}

func TestAddHandlerPanicsIfImplDoesntMatchInterface(t *testing.T) {
	idl := parseTestIdl()
	svr := NewServer(idl)

	badHandlers := []interface{}{
		BImpl_MissingFunc{},
		BImpl_BadParam{},
		BImpl_BadReturn{},
		BImpl_BadReturn2{},
		BImpl_BadReturn3{},
	}

	for x, handler := range(badHandlers) {
		fx := func() {
			defer func() {
				if r := recover(); r != nil {
					// ok
				}
			}()
			svr.AddHandler("B", handler)
			t.Errorf("[%d] - AddHandler() allowed invalid handler impl", x)
		}
		fx()
	}
}

///////////////////////////////

func BenchmarkConvertSlice(b *testing.B) {
	b.StopTimer()
	idl := &Idl{structs: map[string]*Struct{}, enums: map[string][]EnumValue{}}
	arrField := &Field{Type: "float", Optional: false, IsArray: true}

	cases := []ConvertTest{
		ConvertTest{[]float64{}, []interface{}{1, 2.1, 3, 30.3, 32.0, 32323.3, 1, 2.1, 3, 30.3, 32.0, 32323.3}, arrField, true},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for _, test := range cases {
			targetType := reflect.TypeOf(test.target)
			conv := NewConvert(idl, test.field, targetType, test.input, "", false)
			_, err := conv.Run()
			if err != nil {
				panic(err)
			}
		}
	}
}

func BenchmarkConvertString(b *testing.B) {
	b.StopTimer()
	idl := &Idl{structs: map[string]*Struct{}, enums: map[string][]EnumValue{}}
	strField := &Field{Type: "string", Optional: false, IsArray: false}

	cases := []ConvertTest{
		ConvertTest{"hi", "hi", strField, true},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for _, test := range cases {
			targetType := reflect.TypeOf(test.target)
			conv := NewConvert(idl, test.field, targetType, test.input, "", false)
			_, err := conv.Run()
			if err != nil {
				panic(err)
			}
		}
	}
}

func BenchmarkConvertStruct(b *testing.B) {
	b.StopTimer()
	idl := &Idl{structs: map[string]*Struct{}, enums: map[string][]EnumValue{}}
	noNestStruct := &Struct{Name: "NoNesting", Fields: []Field{
		Field{Name: "a", Type: "string", Optional: true, IsArray: false},
		Field{Name: "b", Type: "int", Optional: true, IsArray: false},
		Field{Name: "C", Type: "float", Optional: true, IsArray: false},
		Field{Name: "d", Type: "bool", Optional: true, IsArray: false},
		Field{Name: "E", Type: "string", Optional: true, IsArray: true},
	}}
	noNestField := &Field{Type: "NoNesting", Optional: false, IsArray: true}
	idl.structs["NoNesting"] = noNestStruct

	cases := []ConvertTest{
		ConvertTest{NoNesting{A: "hi", B: 30}, map[string]interface{}{"a": "hi", "b": 30}, noNestField, true},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for _, test := range cases {
			targetType := reflect.TypeOf(test.target)
			conv := NewConvert(idl, test.field, targetType, test.input, "", false)
			_, err := conv.Run()
			if err != nil {
				panic(err)
			}
		}
	}
}
