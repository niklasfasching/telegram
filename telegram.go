package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type API interface {
	Start() error
	Stop()
	Send(v interface{}) error
	Handle(kind string, handlerFunc interface{})
	User() User
}

type Connection struct {
	Token    string
	Timeout  time.Duration
	Debug    bool
	handlers map[string]reflect.Value
	user     User
	offset   int
	stopped  bool
}

type response struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
}

func (c *Connection) User() User { return c.user }

func (c *Connection) Start() error {
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	user := User{}
	if err := c.Call("getMe", nil, &user); err != nil {
		return err
	}
	c.user = user
	if c.Debug {
		log.Println("Started:", prettyPrintJSON(c.user))
	}
	for !c.stopped {
		c.handleUpdates()
	}
	return nil
}

func (c *Connection) Stop() { c.stopped = true }

func (c *Connection) Call(method string, data, result interface{}) error {
	if data == nil {
		data = map[string]interface{}{}
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", c.Token, method)
	res, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	bs, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	debugLog(c.Debug, method, bs)
	r := response{}
	if err = json.Unmarshal(bs, &r); err != nil {
		return err
	}
	if !r.OK {
		return fmt.Errorf("%s (%d) (%s: %s)", r.Description, r.ErrorCode, method, prettyPrintJSON(data))
	}
	return json.Unmarshal(r.Result, result)
}

func (c *Connection) handleUpdates() error {
	updates, data := []map[string]json.RawMessage{}, map[string]interface{}{
		"offset":  c.offset,
		"timeout": c.Timeout.Seconds(),
	}
	if err := c.Call("getUpdates", data, &updates); err != nil {
		return err
	}
	for _, u := range updates {
		offset, err := strconv.Atoi(string(u["update_id"]))
		if err != nil {
			return err
		}
		c.offset = offset + 1
		if err := c.handleUpdate(u); err != nil {
			return err
		}
	}
	return nil
}

func (c *Connection) handleUpdate(update map[string]json.RawMessage) error {
	for kind, handler := range c.handlers {
		if update[kind] == nil {
			continue
		}
		debugLog(c.Debug, kind, []byte(prettyPrintJSON(update)))
		v := reflect.New(handler.Type().In(0))
		if err := json.Unmarshal(update[kind], v.Interface()); err != nil {
			return err
		}
		if err := handler.Call([]reflect.Value{v.Elem()})[0].Interface(); err != nil {
			return err.(error)
		}
		return nil
	}
	debugLog(c.Debug, "unhandled", []byte(prettyPrintJSON(update)))
	return nil
}

func (c *Connection) Handle(kind string, handlerFunc interface{}) {
	v := reflect.ValueOf(handlerFunc)
	if t := v.Type(); t.NumIn() != 1 || t.NumOut() != 1 || t.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
		panic(fmt.Errorf("handlerFunc must be in the format func(T) error"))
	}
	if _, ok := c.handlers[kind]; ok {
		panic(fmt.Errorf("handler for event kind %s has already been registered", kind))
	}
	if c.handlers == nil {
		c.handlers = map[string]reflect.Value{}
	}
	c.handlers[kind] = v
}

func debugLog(debug bool, prefix string, bytes []byte) {
	if !debug {
		return
	}
	m := map[string]interface{}{}
	if err := json.Unmarshal(bytes, &m); err != nil {
		log.Println(prefix, string(bytes))
	} else {
		log.Println(prefix, prettyPrintJSON(m))
	}
}

func prettyPrintJSON(v interface{}) string {
	out := strings.Builder{}
	json := json.NewEncoder(&out)
	json.SetEscapeHTML(false)
	json.SetIndent("", "  ")
	json.Encode(v)
	return out.String()
}
