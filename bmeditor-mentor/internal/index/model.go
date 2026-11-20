// Package index is what the extension knows about a repository's code,
// whatever its language: every declaration with its place and shape, and
// the relations between them (who calls, constructs, references, embeds
// or implements what). A Source fills it: the Go one from the type
// checker, others from a language server. Nothing here guesses at what
// code means.
package index

import "time"

// Kind is what a symbol declares.
type Kind string

const (
	KindStruct    Kind = "struct"
	KindInterface Kind = "interface"
	KindType      Kind = "type" // any other named type
	KindFunc      Kind = "func"
	KindMethod    Kind = "method"
	KindField     Kind = "field"
	KindConst     Kind = "const"
	KindVar       Kind = "var"
)

// Symbol is one declaration at package level, a method, or a struct's
// field. Key names it across edits that only move it: the import path, the
// receiver or owner type for methods and fields, and the name.
type Symbol struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Kind    Kind   `json:"kind"`
	Pkg     string `json:"pkg"` // import path
	PkgName string `json:"pkgName"`
	Recv    string `json:"recv,omitempty"` // the receiver of a method, the owner of a field
	File    string `json:"file"`
	Line    int    `json:"line"` // 1-based, of the name
	Col     int    `json:"col"`
	EndLine int    `json:"endLine"` // the end of the declaration
	Start   int    `json:"start"`   // the declaration's first line, its doc comment left out
	Sig     string `json:"sig"`
	Doc     string `json:"doc,omitempty"`
	// Hash is the declaration's text with its spacing folded, so a change
	// to what it does changes it and gofmt does not.
	Hash         string  `json:"hash"`
	Exported     bool    `json:"exported"`
	Test         bool    `json:"test,omitempty"`      // declared in a _test.go file
	Generated    bool    `json:"generated,omitempty"` // in a file marked "Code generated"
	ReturnsError bool    `json:"returnsError,omitempty"`
	Branches     int     `json:"branches,omitempty"`   // if, switch and select statements
	ErrReturns   int     `json:"errReturns,omitempty"` // returns that pass back an error that is not nil
	Fields       []Field `json:"fields,omitempty"`     // a struct's

	// MethodSet is a type's methods and IfaceMethods an interface's, each
	// as name and signature written the same way, so implements can be
	// worked out across packages read separately. A Source that cannot
	// say leaves them empty and gives implements relations itself.
	MethodSet    []string `json:"-"`
	IfaceMethods []string `json:"-"`
}

// Field is a struct's field as its card shows it.
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Embedded bool   `json:"embedded,omitempty"`
	Tag      string `json:"tag,omitempty"`
}

// RelKind is how one symbol relates to another.
type RelKind string

const (
	RelCalls        RelKind = "calls"
	RelCallsDynamic RelKind = "calls_dynamic" // through an interface: which implementation runs is decided at run time
	RelReferences   RelKind = "references"
	RelConstructs   RelKind = "constructs" // a composite literal of the type
	RelReturns      RelKind = "returns"    // a function whose results include the type
	RelEmbeds       RelKind = "embeds"
	RelImplements   RelKind = "implements" // From's method set covers interface To
	RelFieldOf      RelKind = "field_of"
	RelMethodOf     RelKind = "method_of"
)

// Certainty says how far a relation can be trusted. Verified comes from
// the type checker; dynamic is a call through an interface; inferred is
// from a pattern in the code, such as a route string.
const (
	Verified = "verified"
	Dynamic  = "dynamic"
	Inferred = "inferred"
)

// Relation is one fact between two symbols, with where it was seen.
type Relation struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	Kind      RelKind `json:"kind"`
	File      string  `json:"file,omitempty"`
	Line      int     `json:"line,omitempty"`
	Col       int     `json:"col,omitempty"`
	Certainty string  `json:"certainty"`
}

// Route is an HTTP route registered with a literal path, found by the
// shape of the call (mux.HandleFunc("/login", h.Login)), so inferred.
type Route struct {
	Method  string `json:"method,omitempty"`
	Path    string `json:"path"`
	Handler string `json:"handler,omitempty"` // the handler's key when it is a function here
	Via     string `json:"via"`               // the registering call, such as HandleFunc
	File    string `json:"file"`
	Line    int    `json:"line"`
}

// Package is what one package declares and relates: a Go package, or a
// folder of another language's files.
type Package struct {
	Path      string     `json:"path"`
	Language  string     `json:"language"`
	Name      string     `json:"name"`
	Dir       string     `json:"dir"`
	Module    string     `json:"module"`
	Doc       string     `json:"doc,omitempty"`
	Files     []string   `json:"files"`
	Imports   []string   `json:"imports"`
	Symbols   []*Symbol  `json:"-"`
	Relations []Relation `json:"-"`
	Routes    []Route    `json:"-"`
	Errors    []string   `json:"errors,omitempty"`
	// Stamp is the newest modification time of its files when it was read.
	Stamp time.Time `json:"-"`
}

// Module is a go.mod the repository holds.
type Module struct {
	Path string `json:"path"`
	Dir  string `json:"dir"`
}
