package httpapi

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
)

type sendCodeReq struct {
	Email string `json:"email" binding:"required,email"`
	Scene string `json:"scene" binding:"required,oneof=register reset_password change_email"`
}

func (h *Handler) sendCode(c *gin.Context) {
	var req sendCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.SendCode(c.Request.Context(), req.Email, req.Scene); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

type registerReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Code     string `json:"code" binding:"required,len=6"`
}

func (h *Handler) register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	u, err := h.svc.Register(c.Request.Context(), req.Email, req.Password, req.Code)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"id": u.ID, "email": u.Email}})
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	TOTP     string `json:"totp"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	pair, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, req.TOTP, c.ClientIP(), c.GetHeader("User-Agent"), c.GetHeader("Accept-Language"))
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": pair})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *Handler) refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": pair})
}

func (h *Handler) logout(c *gin.Context) {
	claims := ClaimsFrom(c)
	if err := h.svc.Logout(c.Request.Context(), claims); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *Handler) me(c *gin.Context) {
	claims := ClaimsFrom(c)
	u, err := h.svc.Me(c.Request.Context(), claims.UID)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"id":                  u.ID,
		"email":               u.Email,
		"uuid":                u.UUID,
		"role":                u.Role,
		"status":              u.Status,
		"balance_cny":         u.BalanceCNY,
		"plan_id":             u.PlanID,
		"plan_expire_at":      u.PlanExpireAt,
		"traffic_total":       u.TrafficTotal,
		"traffic_used":        u.TrafficUsed,
		"traffic_reset_at":    u.TrafficResetAt,
		"device_limit":        u.DeviceLimit,
		"subscription_token":  u.SubscriptionToken,
		"totp_enabled":        u.TOTPEnabled,
		"invite_code":         u.InviteCode,
		"last_login_at":       u.LastLoginAt,
		"created_at":          u.CreatedAt,
	}})
}

type changePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (h *Handler) changePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), ClaimsFrom(c).UID, req.OldPassword, req.NewPassword); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *Handler) totpEnroll(c *gin.Context) {
	res, err := h.svc.EnrollTOTP(c.Request.Context(), ClaimsFrom(c).UID)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"secret":     res.Secret,
		"otpauth":    res.URL,
		"qr_png_b64": base64.StdEncoding.EncodeToString(res.QRPNG),
	}})
}

type totpCodeReq struct {
	Code string `json:"code" binding:"required,len=6"`
}

func (h *Handler) totpVerify(c *gin.Context) {
	var req totpCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.VerifyTOTP(c.Request.Context(), ClaimsFrom(c).UID, req.Code); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *Handler) totpDisable(c *gin.Context) {
	var req totpCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4000, "message": err.Error()})
		return
	}
	if err := h.svc.DisableTOTP(c.Request.Context(), ClaimsFrom(c).UID, req.Code); err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}
