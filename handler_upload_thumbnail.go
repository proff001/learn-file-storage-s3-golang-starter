package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	const maxMemory = 10 << 20 // 10 MB
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	contentHeader := header.Header.Get("Content-Type")
	if contentHeader == "" {
		respondWithError(w, http.StatusBadRequest, "Couldn't find media type", err)
		return
	}

	mediaType, _, err := mime.ParseMediaType(contentHeader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Unsupported media type", err)
		return
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read file", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You do not have permission to upload a thumbnail for this video", nil)
		return
	}

	fileExtension, err := mime.ExtensionsByType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find file extension for media type", err)
		return
	}

	randBytes := make([]byte, 32)
	rand.Read(randBytes)

	fileName := base64.RawURLEncoding.EncodeToString(randBytes) + fileExtension[0]
	filePath := filepath.Join(cfg.assetsRoot, fileName)
	err = os.WriteFile(filePath, fileData, 0644)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write thumbnail to file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update thumbnail", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
