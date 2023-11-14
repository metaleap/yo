//go:build debug

package yopenapi

// yoValiOnly yoFail

const Version = "3.1.0"

type OpenApi struct {
	OpenApi string          `json:"openapi"`
	Info    Info            `json:"info"`
	Paths   map[string]Path `json:"paths"`
}

type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
	Contact struct {
		Url string `json:"url"`
	} `json:"contact"`
}

type Path struct {
	Post Op `json:"post"`
}

type Op struct {
	Id         string          `json:"operationId"`
	Summary    string          `json:"summary,omitempty"`
	Descr      string          `json:"description,omitempty"`
	Deprecated bool            `json:"deprecated,omitempty"`
	Params     []Param         `json:"parameters"`
	ReqBody    ReqBody         `json:"requestBody"`
	Responses  map[string]Resp `json:"responses"`
}

type Param struct {
	Name       string           `json:"name"`
	In         string           `json:"in"` // query|header|cookie
	Descr      string           `json:"description,omitempty"`
	Required   bool             `json:"required,omitempty"`
	Deprecated bool             `json:"deprecated,omitempty"`
	Content    map[string]Media `json:"content"`
}

type ReqBody struct {
	Descr    string           `json:"description,omitempty"`
	Required bool             `json:"required,omitempty"`
	Content  map[string]Media `json:"content"`
}

type Resp struct {
	Descr   string            `json:"description"`
	Headers map[string]Header `json:"headers"`
	Content map[string]Media  `json:"content"`
}

type Header struct {
	Descr      string `json:"description,omitempty"`
	Required   bool   `json:"required,omitempty"`
	Deprecated bool   `json:"deprecated,omitempty"`
}

type Media struct {
	Example any `json:"example"`
}
