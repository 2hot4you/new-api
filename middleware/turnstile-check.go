package middleware

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type turnstileCheckResponse struct {
	Success bool `json:"success"`
}

var turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

func TurnstileCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.TurnstileCheckEnabled {
			response := c.Query("turnstile")
			if response == "" {
				abortWithApplicationError(c, http.StatusOK, "TURNSTILE_REQUIRED", "Turnstile token 为空")
				return
			}
			rawRes, err := http.PostForm(turnstileVerifyURL, url.Values{
				"secret":   {common.TurnstileSecretKey},
				"response": {response},
				"remoteip": {c.ClientIP()},
			})
			if err != nil {
				common.SysLog(fmt.Sprintf("Turnstile verification request failed: %v", err))
				abortWithApplicationError(c, http.StatusOK, "TURNSTILE_UNAVAILABLE", "Turnstile 服务暂时不可用，请稍后重试！")
				return
			}
			defer rawRes.Body.Close()
			if rawRes.StatusCode < http.StatusOK || rawRes.StatusCode >= http.StatusMultipleChoices {
				common.SysLog(fmt.Sprintf("Turnstile verification returned HTTP %d", rawRes.StatusCode))
				abortWithApplicationError(c, http.StatusOK, "TURNSTILE_UNAVAILABLE", "Turnstile 服务暂时不可用，请稍后重试！")
				return
			}
			var res turnstileCheckResponse
			err = common.DecodeJson(rawRes.Body, &res)
			if err != nil {
				common.SysLog(fmt.Sprintf("Turnstile verification response decode failed: %v", err))
				abortWithApplicationError(c, http.StatusOK, "TURNSTILE_UNAVAILABLE", "Turnstile 服务暂时不可用，请稍后重试！")
				return
			}
			if !res.Success {
				abortWithApplicationError(c, http.StatusOK, "TURNSTILE_INVALID", "Turnstile 校验失败，请刷新重试！")
				return
			}
		}
		c.Next()
	}
}
