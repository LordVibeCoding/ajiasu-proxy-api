package api

import (
	"encoding/json"
	"net/http"

	"ajiasu-proxy-api/internal/ajiasu"
)

type AjiasuHandler struct {
	mgr *ajiasu.Manager
}

func NewAjiasuHandler(mgr *ajiasu.Manager) *AjiasuHandler {
	return &AjiasuHandler{mgr: mgr}
}

func (h *AjiasuHandler) Status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.mgr.Status())
}

func (h *AjiasuHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.mgr.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "count": len(nodes)})
}

func (h *AjiasuHandler) Connect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Node string `json:"node"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Node == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "需要 node 参数"})
		return
	}
	if err := h.mgr.Connect(req.Node); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    "已连接",
		"node":       req.Node,
		"proxy_addr": h.mgr.ProxyAddr(),
	})
}

func (h *AjiasuHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	h.mgr.Disconnect()
	writeJSON(w, http.StatusOK, map[string]any{"message": "已断开"})
}

func (h *AjiasuHandler) AutoSelect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cities []string `json:"cities"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	result, err := h.mgr.AutoSelect(req.Cities)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"node":       result.Node,
		"delay_ms":   result.DelayMs,
		"ip":         result.IP,
		"proxy_addr": result.ProxyAddr,
		"message":    "智能选节点成功",
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
