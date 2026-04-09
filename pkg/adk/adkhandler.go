package adk

//
// import (
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"strings"
// )
//
// // POST /api/signal/session  — start a session
// // POST /api/signal/turn     — stream a turn
//
// type createSessionRequest struct {
// 	SessionID    string `json:"session_id"`
// 	QuestionBank string `json:"question_bank"`
// }
//
// type createSessionResponse struct {
// 	SessionID string `json:"session_id"`
// }
//
// type processTurnRequest struct {
// 	SessionID  string `json:"session_id"`
// 	Transcript string `json:"transcript"`
// }
//
// // HandleCreateSession starts a new interview session.
// func (s *ADKService) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}
// 	var req createSessionRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
// 		return
// 	}
// 	if strings.TrimSpace(req.SessionID) == "" {
// 		http.Error(w, "session_id is required", http.StatusBadRequest)
// 		return
// 	}
// 	if strings.TrimSpace(req.QuestionBank) == "" {
// 		http.Error(w, "question_bank is required", http.StatusBadRequest)
// 		return
// 	}
//
// 	if err := s.SignalingAgent().StartSession(r.Context(), req.SessionID, req.QuestionBank); err != nil {
// 		http.Error(w, fmt.Sprintf("start session: %v", err), http.StatusInternalServerError)
// 		return
// 	}
//
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusCreated)
// 	_ = json.NewEncoder(w).Encode(createSessionResponse{SessionID: req.SessionID})
// }
//
// // HandleProcessTurn feeds one transcript chunk and streams the signal back.
// func (s *ADKService) HandleProcessTurn(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}
// 	var req processTurnRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
// 		return
// 	}
// 	if strings.TrimSpace(req.SessionID) == "" {
// 		http.Error(w, "session_id is required", http.StatusBadRequest)
// 		return
// 	}
// 	if strings.TrimSpace(req.Transcript) == "" {
// 		http.Error(w, "transcript is required", http.StatusBadRequest)
// 		return
// 	}
//
// 	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
// 	w.Header().Set("Transfer-Encoding", "chunked")
// 	w.Header().Set("X-Accel-Buffering", "no")
// 	w.WriteHeader(http.StatusOK)
//
// 	flusher, canFlush := w.(http.Flusher)
//
// 	for chunk, err := range s.SignalingAgent().SendTurn(r.Context(), req.SessionID, req.Transcript) {
// 		if err != nil {
// 			fmt.Fprintf(w, "\nERROR: %v\n", err)
// 			if canFlush {
// 				flusher.Flush()
// 			}
// 			return
// 		}
// 		fmt.Fprint(w, chunk)
// 		if canFlush {
// 			flusher.Flush()
// 		}
// 	}
// }
