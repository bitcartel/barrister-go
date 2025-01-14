package barrister

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

var reservedWords []string = []string{
	"break", "default", "func", "interface", "select",
	"case", "defer", "go", "map", "struct",
	"chan", "else", "goto", "package", "switch",
	"const", "fallthrough", "if", "range", "type",
	"continue", "for", "import", "return", "var",
}

func escReserved(s string) string {
	for _, word := range reservedWords {
		if word == s {
			return "_" + s
		}
	}
	return s
}

type generateGo struct {
	// full IDL from JSON file
	idl *Idl

	// sub-set of IDL that only contains elements in this package
	pkgIdl *Idl

	// go package name for this file
	pkgName string

	// if true, [optional] fields will be generated as pointers
	// if false, "omitempty" will be added to the json tag
	optionalToPtr bool

	// imports to add
	imports []string

	// base import string to prefix to imports values
	baseImport string
}

func (g *generateGo) hasInterface() bool {
	return len(g.pkgIdl.interfaces) > 0
}

func (g *generateGo) generate() []byte {
	b := &bytes.Buffer{}
	line(b, 0, fmt.Sprintf("package %s\n", g.pkgName))
	line(b, 0, "import (")
	if g.hasInterface() {
		line(b, 1, `"fmt"`)
		line(b, 1, `"reflect"`)
		line(b, 1, `"github.com/coopernurse/barrister-go"`)
	}
	for _, imp := range g.imports {
		line(b, 1, fmt.Sprintf("\"%s%s\"", g.baseImport, imp))
	}
	line(b, 0, ")\n")

	if g.hasInterface() {
		line(b, 0, "const BarristerVersion string = \""+g.idl.Meta.BarristerVersion+"\"")
		line(b, 0, "const BarristerChecksum string = \""+g.idl.Meta.Checksum+"\"")
		line(b, 0, fmt.Sprintf("const BarristerDateGenerated int64 = %d", g.idl.Meta.DateGenerated))
		line(b, 0, "")
	}

	for name, _ := range g.pkgIdl.enums {
		g.generateEnum(b, name)
	}

	for _, elem := range g.pkgIdl.elems {
		if elem.Type == "struct" {
			s, ok := g.pkgIdl.structs[elem.Name]
			if !ok {
				panic("No struct found: " + elem.Name)
			}
			g.generateStruct(b, s)
		}
	}
	line(b, 0, "")

	if g.hasInterface() {
		for _, name := range sortedKeys(g.pkgIdl.interfaces) {
			g.generateInterface(b, name)
			line(b, 0, "}\n")
			g.generateProxy(b, name)
		}

		g.generateNewServer(b)
		g.generateIdlJson(b)
	}

	return b.Bytes()
}

func (g *generateGo) generateIdlJson(b *bytes.Buffer) {
	idlbytes, err := json.MarshalIndent(g.idl.elems, "", "    ")
	if err != nil {
		panic(err)
	}
	idlstr := strings.Replace(string(idlbytes), "`", "`+\"`\"+`", -1)
	line(b, 0, "")
	line(b, 0, "var IdlJsonRaw = `"+idlstr+"`")
}

func (g *generateGo) generateEnum(b *bytes.Buffer, enumName string) {
	vals, ok := g.idl.enums[enumName]
	if !ok {
		panic("No enum found: " + enumName)
	}

	goName := capitalizeAndStripMatchingPkg(enumName, g.pkgName)
	line(b, 0, fmt.Sprintf("type %s string", goName))
	line(b, 0, "const (")
	for x, val := range vals {
		typeStr := ""
		if x == 0 {
			typeStr = goName
		}
		line(b, 1, fmt.Sprintf("%s%s %s = \"%s\"",
			goName, capitalize(val.Value), typeStr, val.Value))
	}
	line(b, 0, ")\n")
}

func (g *generateGo) generateStruct(b *bytes.Buffer, s *Struct) {
	goName := capitalizeAndStripMatchingPkg(s.Name, g.pkgName)
	line(b, 0, fmt.Sprintf("type %s struct {", goName))
	if s.Extends != "" {
		line(b, 1, capitalizeAndStripMatchingPkg(s.Extends, g.pkgName))
	}
	for _, f := range s.Fields {
		goName = capitalize(f.Name)
		omit := ""
		if f.Optional {
			omit = ",omitempty"
		}
		line(b, 1, fmt.Sprintf("%s\t%s\t`json:\"%s%s\"`",
			goName, f.goType(g.idl, g.optionalToPtr, g.pkgName), f.Name, omit))
	}
	line(b, 0, "}\n")
}

func (g *generateGo) generateNewServer(b *bytes.Buffer) {
	ifaceKeys := sortedKeys(g.idl.interfaces)
	ifaces := ""
	ifaceIdents := ""
	for _, name := range ifaceKeys {
		upper := capitalize(name)
		lower := escReserved(strings.ToLower(name))
		ifaces = fmt.Sprintf("%s, %s %s", ifaces, lower, upper)
		ifaceIdents += ", " + lower
	}

	line(b, 0, fmt.Sprintf("func NewJSONServer(idl *barrister.Idl, forceASCII bool%s) barrister.Server {", ifaces))
	line(b, 1, fmt.Sprintf("return NewServer(idl, &barrister.JsonSerializer{forceASCII}%s)", ifaceIdents))
	line(b, 0, "}\n")

	line(b, 0, fmt.Sprintf("func NewServer(idl *barrister.Idl, ser barrister.Serializer%s) barrister.Server {", ifaces))
	line(b, 1, fmt.Sprintf("_svr := barrister.NewServer(idl, ser)"))
	for _, name := range ifaceKeys {
		lower := strings.ToLower(name)
		line(b, 1, fmt.Sprintf("_svr.AddHandler(\"%s\", %s)", name, lower))
	}
	line(b, 1, "return _svr")
	line(b, 0, "}")
}

func (g *generateGo) generateInterface(b *bytes.Buffer, ifaceName string) {
	funcs, ok := g.idl.interfaces[ifaceName]
	if !ok {
		panic("No interface found: " + ifaceName)
	}

	goName := capitalize(ifaceName)
	line(b, 0, fmt.Sprintf("type %s interface {", goName))
	for _, fn := range funcs {
		goName = capitalize(fn.Name)
		params := ""
		for x, p := range fn.Params {
			if x > 0 {
				params += ", "
			}
			params += fmt.Sprintf("%s %s", escReserved(p.Name), p.goType(g.idl, g.optionalToPtr, g.pkgName))
		}
		line(b, 1, fmt.Sprintf("%s(%s) (%s, error)",
			goName, params, fn.Returns.goType(g.idl, g.optionalToPtr, g.pkgName)))
	}
}

func (g *generateGo) generateProxy(b *bytes.Buffer, ifaceName string) {
	funcs, ok := g.idl.interfaces[ifaceName]
	if !ok {
		panic("No interface found: " + ifaceName)
	}

	goIfaceName := capitalize(ifaceName)
	goName := goIfaceName + "Proxy"

	line(b, 0, fmt.Sprintf("func New%s(c barrister.Client) %s { return %s{c, barrister.MustParseIdlJson([]byte(IdlJsonRaw))} }\n", goName, goIfaceName, goName))

	line(b, 0, fmt.Sprintf("type %s struct {", goName))
	line(b, 1, "client barrister.Client")
	line(b, 1, "idl    *barrister.Idl")
	line(b, 0, "}\n")
	for _, fn := range funcs {
		method := fmt.Sprintf("%s.%s", ifaceName, fn.Name)
		retType := fn.Returns.goType(g.idl, g.optionalToPtr, g.pkgName)
		zeroVal := fn.Returns.zeroVal(g.idl, g.optionalToPtr, g.pkgName)
		fnName := capitalize(fn.Name)
		params := ""
		paramIdents := ""
		for x, p := range fn.Params {
			if x > 0 {
				params += ", "
			}
			ident := escReserved(p.Name)
			params += fmt.Sprintf("%s %s", ident, p.goType(g.idl, g.optionalToPtr, g.pkgName))
			paramIdents += ", "
			paramIdents += ident
		}
		line(b, 0, fmt.Sprintf("func (_p %s) %s(%s) (%s, error) {",
			goName, fnName, params, retType))
		line(b, 1, fmt.Sprintf("_res, _err := _p.client.Call(\"%s\"%s)",
			method, paramIdents))
		line(b, 1, "if _err == nil {")
		if g.optionalToPtr && fn.Returns.Optional {
			line(b, 2, "if _res == nil {")
			line(b, 3, "return nil, nil")
			line(b, 2, "}")
		}
		line(b, 2, fmt.Sprintf("_retType := _p.idl.Method(\"%s\").Returns", method))
		line(b, 2, fmt.Sprintf("_res, _err = barrister.Convert(_p.idl, &_retType, reflect.TypeOf(%s), _res, \"\")", zeroVal))
		line(b, 1, "}")
		line(b, 1, "if _err == nil {")
		line(b, 2, fmt.Sprintf("_cast, _ok := _res.(%s)", retType))
		line(b, 2, "if !_ok {")
		line(b, 3, "_t := reflect.TypeOf(_res)")
		line(b, 3, `_msg := fmt.Sprintf("`+method+` returned invalid type: %v", _t)`)
		line(b, 3, fmt.Sprintf("return %s, &barrister.JsonRpcError{Code: -32000, Message: _msg}", zeroVal))
		line(b, 2, "}")
		line(b, 2, "return _cast, nil")
		line(b, 1, "}")
		line(b, 1, fmt.Sprintf("return %s, _err", zeroVal))
		line(b, 0, "}\n")
	}
}

func comment(b *bytes.Buffer, level int, comment string) {
	if comment != "" {
		for _, ln := range strings.Split(comment, "\n") {
			line(b, level, fmt.Sprintf("// %s", ln))
		}
	}
}

func line(b *bytes.Buffer, level int, s string) {
	for i := 0; i < level; i++ {
		b.WriteString("\t")
	}
	b.WriteString(s)
	b.WriteString("\n")
}

func sortedKeys(m map[string][]Function) []string {
	mk := make([]string, len(m))
	i := 0
	for k, _ := range m {
		mk[i] = k
		i++
	}
	sort.Strings(mk)
	return mk
}
