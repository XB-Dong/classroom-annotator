package help

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func (s *Service) RegisterDocument(r *gin.Engine) {
	r.GET("/api/v1/help/document", func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.JSON(200, s.Document) })
}

// Register belongs to the already-authenticated and CSRF-protected API group.
func (s *Service) Register(g *gin.RouterGroup) {
	g.POST("/help/questions", func(c *gin.Context) {
		actor, ok := auth.ActorFrom(c.Request.Context())
		if !ok {
			helpError(c, 401, "AUTH_REQUIRED", "请先登录。")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
		var body struct {
			Question string `json:"question"`
		}
		dec := json.NewDecoder(c.Request.Body)
		dec.DisallowUnknownFields()
		if dec.Decode(&body) != nil {
			helpError(c, 400, "INVALID_REQUEST", "问题格式无效。")
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			helpError(c, 400, "INVALID_REQUEST", "问题格式无效。")
			return
		}
		answer, e := s.Ask(c.Request.Context(), actor.ID, body.Question)
		switch e {
		case nil:
			c.JSON(200, answer)
		case ErrInvalidQuestion:
			helpError(c, 422, "INVALID_QUESTION", "请填写1000字以内的平台操作问题。")
		case ErrLimited:
			helpError(c, 429, "HELP_RATE_LIMITED", "问答繁忙或请求过多，请稍后再试。")
		case ErrInvalidResponse:
			helpError(c, 502, "HELP_INVALID_RESPONSE", "指南助手回答未通过检查，请查阅完整手册。")
		default:
			helpError(c, 503, "HELP_UNAVAILABLE", "指南助手暂不可用，请查阅完整手册。")
		}
	})
}
func helpError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": message, "requestId": c.GetString("requestId")})
}
