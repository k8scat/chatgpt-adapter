package interpreter

import (
	"github.com/bincooo/chatgpt-adapter/internal/common"
	"github.com/bincooo/chatgpt-adapter/internal/gin.handler/response"
	"github.com/bincooo/chatgpt-adapter/internal/plugin"
	"github.com/bincooo/chatgpt-adapter/logger"
	"github.com/bincooo/chatgpt-adapter/pkg"
	"github.com/gin-gonic/gin"
	socketio "github.com/zishang520/socket.io/socket"
	"sync"
)

// OpenInterpreter/open-interpreter
var (
	Adapter = API{}
	Model   = "open-interpreter"

	mu sync.Mutex
	ws *socketio.Socket
)

type API struct {
	plugin.BaseAdapter
}

func init() {
	common.AddInitialized(func() {
		if !pkg.Config.GetBool("interpreter.ws") {
			return
		}

		err := plugin.IO.On("connection", func(events ...any) {
			if len(events) == 0 {
				return
			}

			w, ok := events[0].(*socketio.Socket)
			if !ok {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			if !initSocketIO(w) {
				w.Disconnect(true)
				return
			}
			logger.Infof("connection event: %v", w)
		})
		if err != nil {
			logger.Errorf("socket.io connection event error: %v", err)
		}
	})
}

func (API) Match(_ *gin.Context, model string) bool {
	return model == Model
}

func (API) Models() []plugin.Model {
	return []plugin.Model{
		{
			Id:      "open-interpreter",
			Object:  "model",
			Created: 1686935002,
			By:      "interpreter-adapter",
		},
	}
}

func (API) Completion(ctx *gin.Context) {
	var (
		proxies    = ctx.GetString("proxies")
		completion = common.GetGinCompletion(ctx)
		matchers   = common.GetGinMatchers(ctx)
	)

	r, tokens, err := fetch(ctx, proxies, completion)
	if err != nil {
		logger.Error(err)
		response.Error(ctx, -1, err)
		return
	}

	ctx.Set(ginTokens, tokens)
	content := waitResponse(ctx, matchers, r, completion.Stream)
	if content == "" && response.NotResponse(ctx) {
		response.Error(ctx, -1, "EMPTY RESPONSE")
	}
}

func initSocketIO(w *socketio.Socket) bool {
	if ws != nil {
		return false
	}

	r := w.Request()
	if token := r.GetPathInfo(); token != "/socket.io/open-i/" {
		return false
	}

	w.On("disconnect", func(...any) {
		mu.Lock()
		defer mu.Unlock()
		ws = nil
	})

	w.On("ping", func(...any) {
		w.Emit("pong", "ok")
	})

	w.On("message", func(args ...any) {
		message := args[0].(string)
		// TODO -
		logger.Infof("message: %s", message)
		w.Emit("message", "ok")
	})

	ws = w
	return true
}
