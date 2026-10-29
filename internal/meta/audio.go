package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

// TranscribeAudio converts audio bytes to text using muse-voice-transcribe-1.0
// via the official /v1/asr/transcribe endpoint.
// Messenger voice notes arrive as mp4 — must convert to WAV first.
func (c *Client) TranscribeAudio(ctx context.Context, audio []byte, filename string) (string, error) {
	// Convert mp4/m4a → WAV if needed (the ASR endpoint requires WAV)
	if len(audio) > 16 && !bytes.Contains(audio[:16], []byte("RIFF")) {
		audio = convertToWAV(ctx, audio)
	}

	// Build multipart: request (JSON settings) + audio (WAV file)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// part 1: request JSON settings
	reqJSON := map[string]any{
		"model":        "muse-voice-transcribe-1.0",
		"audioEncoding": "WAV",
		"mode":          "PUSH_TO_TALK",
	}
	reqBytes, _ := json.Marshal(reqJSON)
	if err := w.WriteField("request", string(reqBytes)); err != nil {
		return "", err
	}

	// part 2: audio file
	fw, err := w.CreateFormFile("audio", "recording.wav")
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	// POST to /v1/asr/transcribe
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/asr/transcribe", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("asr request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("asr %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	// Parse the response
	var out struct {
		Transcript string `json:"transcript"`
		Text       string `json:"text"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("asr parse: %w", err)
	}
	if out.Transcript != "" {
		return out.Transcript, nil
	}
	if out.Text != "" {
		return out.Text, nil
	}
	// If the response doesn't have transcript/text, try full body as string
	return string(raw), nil
}

// convertToWAV converts audio bytes (mp4/m4a/etc) to 16kHz mono WAV.
// MP4 needs seekable input — use temp files, not pipes.
func convertToWAV(ctx context.Context, audio []byte) []byte {
	tmpDir := os.TempDir()
	inPath := filepath.Join(tmpDir, fmt.Sprintf("kancil-audio-in-%d.mp4", os.Getpid()))
	outPath := filepath.Join(tmpDir, fmt.Sprintf("kancil-audio-out-%d.wav", os.Getpid()))

	defer func() {
		os.Remove(inPath)
		os.Remove(outPath)
	}()

	if err := os.WriteFile(inPath, audio, 0o644); err != nil {
		return audio
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inPath,
		"-ar", "24000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		"-map_metadata", "-1",
		"-y", outPath,
	)
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return audio
	}

	converted, err := os.ReadFile(outPath)
	if err != nil || len(converted) < 100 {
		return audio
	}
	return converted
}
