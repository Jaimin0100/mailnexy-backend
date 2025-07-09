package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// GenerateTrackingPixelURL generates a tracking pixel URL for email opens
func GenerateTrackingPixelURL(baseURL, messageID string) string {
	token := generateUniqueToken(messageID)
	return fmt.Sprintf("%s/track/open/%s/%s", baseURL, messageID, token)
}

// GenerateClickTrackURL generates a tracked URL for links
func GenerateClickTrackURL(baseURL, messageID, originalURL string) string {
	token := generateUniqueToken(messageID)
	encodedURL := url.QueryEscape(originalURL)
	return fmt.Sprintf("%s/track/click/%s/%s?url=%s", baseURL, messageID, token, encodedURL)
}

// InjectTracking injects tracking into email content
func InjectTracking(htmlContent, baseURL, messageID string) string {
	// Add open tracking pixel
	pixelURL := GenerateTrackingPixelURL(baseURL, messageID)
	trackingPixel := fmt.Sprintf(`<img src="%s" alt="" width="1" height="1" style="display:none">`, pixelURL)
	
	// Inject click tracking for all links
	modifiedHTML := injectClickTracking(htmlContent, baseURL, messageID)
	
	return modifiedHTML + trackingPixel
}

func injectClickTracking(html, baseURL, messageID string) string {
	// This is a simplified version. Consider using an HTML parser for production
	startTag := "<a href=\""
	endTag := "\""
	offset := 0

	for {
		startIdx := strings.Index(html[offset:], startTag)
		if startIdx == -1 {
			break
		}
		startIdx += offset + len(startTag)
		
		endIdx := strings.Index(html[startIdx:], endTag)
		if endIdx == -1 {
			break
		}
		endIdx += startIdx

		originalURL := html[startIdx:endIdx]
		trackedURL := GenerateClickTrackURL(baseURL, messageID, originalURL)
		
		html = html[:startIdx] + trackedURL + html[endIdx:]
		offset = startIdx + len(trackedURL)
	}
	
	return html
}

func generateUniqueToken(messageID string) string {
	hash := sha256.Sum256([]byte(uuid.New().String() + messageID))
	return base64.URLEncoding.EncodeToString(hash[:])[:20]
}
