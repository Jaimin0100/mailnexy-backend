package controller

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"mailnexy/config"
	"mailnexy/models"
	"mailnexy/utils"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UniboxController struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewUniboxController(db *gorm.DB, logger *log.Logger) *UniboxController {
	return &UniboxController{
		db:     db,
		logger: logger,
	}
}

// FetchEmails fetches emails from all connected senders
func (uc *UniboxController) FetchEmails(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	// Get all senders for the user
	var senders []models.Sender
	if err := config.DB.Where("user_id = ?", user.ID).Find(&senders).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch senders",
		})
	}

	// Create system folders if they don't exist
	if err := uc.createSystemFolders(user.ID); err != nil {
		uc.logger.Printf("Failed to create system folders: %v", err)
	}

	// Fetch emails from each sender
	for _, sender := range senders {
		if sender.IMAPHost != "" {
			if err := uc.fetchFromIMAP(&sender, user.ID); err != nil {
				uc.logger.Printf("Failed to fetch emails from sender %d: %v", sender.ID, err)
				continue
			}
		}
	}

	return c.JSON(fiber.Map{
		"message": "Email fetch completed",
	})
}

func (uc *UniboxController) createSystemFolders(userID uint) error {
	systemFolders := []string{"Inbox", "Sent", "Drafts", "Spam", "Trash", "Archive"}

	for _, folderName := range systemFolders {
		var existingFolder models.UniboxFolder
		err := uc.db.Where("user_id = ? AND name = ? AND system = ?", userID, folderName, true).First(&existingFolder).Error
		if err == gorm.ErrRecordNotFound {
			newFolder := models.UniboxFolder{
				UserID: userID,
				Name:   folderName,
				System: true,
			}
			if err := uc.db.Create(&newFolder).Error; err != nil {
				return fmt.Errorf("failed to create folder %s: %v", folderName, err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to check for folder %s: %v", folderName, err)
		}
	}

	return nil
}

func (uc *UniboxController) fetchFromIMAP(sender *models.Sender, userID uint) error {
	password, err := utils.Decrypt(sender.IMAPPassword)
	if err != nil {
		return fmt.Errorf("failed to decrypt IMAP password: %v", err)
	}

	var imapClient *client.Client
	imapAddr := fmt.Sprintf("%s:%d", sender.IMAPHost, sender.IMAPPort)

	switch strings.ToUpper(sender.IMAPEncryption) {
	case "SSL", "TLS":
		imapClient, err = client.DialTLS(imapAddr, &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         sender.IMAPHost,
		})
	case "STARTTLS":
		imapClient, err = client.Dial(imapAddr)
		if err == nil {
			err = imapClient.StartTLS(&tls.Config{
				InsecureSkipVerify: false,
				ServerName:         sender.IMAPHost,
			})
		}
	default:
		imapClient, err = client.Dial(imapAddr)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to IMAP server: %v", err)
	}
	defer imapClient.Logout()

	if err := imapClient.Login(sender.IMAPUsername, password); err != nil {
		return fmt.Errorf("failed to login to IMAP server: %v", err)
	}

	mailbox := "INBOX"
	if sender.IMAPMailbox != "" {
		mailbox = sender.IMAPMailbox
	}

	_, err = imapClient.Select(mailbox, false)
	if err != nil {
		return fmt.Errorf("failed to select mailbox: %v", err)
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{"\\Seen"}
	ids, err := imapClient.Search(criteria)
	if err != nil {
		return fmt.Errorf("failed to search messages: %v", err)
	}

	if len(ids) == 0 {
		return nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	// --- CHANGE MADE HERE ---
	// Replaced imap.FetchRFC822 with imap.FetchRFC822Peek and removed invalid imap.FetchBody
	go func() {
		done <- imapClient.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchItem("BODY.PEEK[]")}, messages)
	}()

	for msg := range messages {
		if err := uc.processIMAPMessage(msg, userID, sender.ID); err != nil {
			uc.logger.Printf("Failed to process message %d: %v", msg.SeqNum, err)
			continue
		}
	}

	if err := <-done; err != nil {
		return fmt.Errorf("error during fetch: %v", err)
	}

	return nil
}

func (uc *UniboxController) processIMAPMessage(msg *imap.Message, userID uint, senderID uint) error {
	// Parse message
	var bodyText, bodyHTML string
	var attachments []string

	if msg.Body != nil {
		// Get the RFC822 message body (entire message)
		section := imap.BodySectionName{}
		literal, ok := msg.Body[&section]
		if !ok {
			return fmt.Errorf("message body not found")
		}

		// Create a reader from the literal
		mr, err := mail.CreateReader(literal)
		if err != nil {
			return fmt.Errorf("failed to create message reader: %v", err)
		}

		// Process each message part
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break // Done with all parts
			} else if err != nil {
				return fmt.Errorf("failed to read next part: %v", err)
			}

			switch h := p.Header.(type) {
			case *mail.InlineHeader:
				contentType, _, _ := h.ContentType()
				b, err := io.ReadAll(p.Body)
				if err != nil {
					return fmt.Errorf("failed to read body: %v", err)
				}

				if strings.Contains(contentType, "text/html") {
					bodyHTML = string(b)
				} else if strings.Contains(contentType, "text/plain") {
					bodyText = string(b)
				}
			case *mail.AttachmentHeader:
				filename, _ := h.Filename()
				attachments = append(attachments, filename)
				// You could save the attachment here if needed
			}
		}
	}

	// Rest of your function remains the same...
	email := models.UniboxEmail{
		UserID:      userID,
		SenderID:    senderID,
		MessageID:   msg.Envelope.MessageId,
		ThreadID:    msg.Envelope.InReplyTo,
		From:        formatAddress(msg.Envelope.From),
		To:          formatAddress(msg.Envelope.To),
		Subject:     msg.Envelope.Subject,
		Body:        bodyText,
		BodyHTML:    bodyHTML,
		Date:        msg.Envelope.Date,
		Attachments: attachments,
	}

	// Save to database
	if err := uc.db.Create(&email).Error; err != nil {
		return fmt.Errorf("failed to save email: %v", err)
	}

	// Add to Inbox folder
	var inboxFolder models.UniboxFolder
	if err := uc.db.Where("user_id = ? AND name = ?", userID, "Inbox").First(&inboxFolder).Error; err != nil {
		return fmt.Errorf("failed to find Inbox folder: %v", err)
	}

	emailFolder := models.UniboxEmailFolder{
		EmailID:  email.ID,
		FolderID: inboxFolder.ID,
	}

	if err := uc.db.Create(&emailFolder).Error; err != nil {
		return fmt.Errorf("failed to add email to folder: %v", err)
	}

	return nil
}

func formatAddress(addrs []*imap.Address) string {
	var result []string
	for _, addr := range addrs {
		if addr.PersonalName != "" {
			result = append(result, fmt.Sprintf("%s <%s>", addr.PersonalName, addr.MailboxName+"@"+addr.HostName))
		} else {
			result = append(result, fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName))
		}
	}
	return strings.Join(result, ", ")
}

// GetEmails returns emails from the unified inbox
func (uc *UniboxController) GetEmails(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	folderName := c.Query("folder", "Inbox")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search")

	// Get folder ID
	var folder models.UniboxFolder
	if err := uc.db.Where("user_id = ? AND name = ?", user.ID, folderName).First(&folder).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Folder not found",
		})
	}

	query := uc.db.Model(&models.UniboxEmail{}).
		Joins("JOIN unibox_email_folders ON unibox_email_folders.email_id = unibox_emails.id").
		Where("unibox_email_folders.folder_id = ?", folder.ID).
		Where("unibox_emails.user_id = ?", user.ID).
		Preload("Sender")

	if search != "" {
		query = query.Where("subject LIKE ? OR body LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count emails",
		})
	}

	var emails []models.UniboxEmail
	offset := (page - 1) * limit
	if err := query.Order("date DESC").Offset(offset).Limit(limit).Find(&emails).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch emails",
		})
	}

	return c.JSON(fiber.Map{
		"data":  emails,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetEmail returns a single email with full details
func (uc *UniboxController) GetEmail(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	emailID := c.Params("id")

	var email models.UniboxEmail
	if err := uc.db.Where("id = ? AND user_id = ?", emailID, user.ID).
		Preload("Sender").
		First(&email).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Email not found",
		})
	}

	// Mark as read
	if !email.IsRead {
		if err := uc.db.Model(&email).Update("is_read", true).Error; err != nil {
			uc.logger.Printf("Failed to mark email as read: %v", err)
		}
	}

	return c.JSON(email)
}

// MoveEmail moves an email to another folder
func (uc *UniboxController) MoveEmail(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	emailID := c.Params("id")
	folderName := c.Query("folder")

	if folderName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Folder name is required",
		})
	}

	// Get folder
	var folder models.UniboxFolder
	if err := uc.db.Where("user_id = ? AND name = ?", user.ID, folderName).First(&folder).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Folder not found",
		})
	}

	// Check if email exists
	var email models.UniboxEmail
	if err := uc.db.Where("id = ? AND user_id = ?", emailID, user.ID).First(&email).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Email not found",
		})
	}

	// Remove from all folders (except for special cases like Archive)
	if folderName != "Archive" {
		if err := uc.db.Where("email_id = ?", email.ID).Delete(&models.UniboxEmailFolder{}).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to remove email from folders",
			})
		}
	}

	// Add to new folder
	emailFolder := models.UniboxEmailFolder{
		EmailID:  email.ID,
		FolderID: folder.ID,
	}

	if err := uc.db.Create(&emailFolder).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to add email to folder",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Email moved successfully",
	})
}

// UpdateEmail updates email properties (read, starred, etc.)
func (uc *UniboxController) UpdateEmail(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	emailID := c.Params("id")

	var req struct {
		IsRead      *bool `json:"is_read"`
		IsStarred   *bool `json:"is_starred"`
		IsImportant *bool `json:"is_important"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Check if email exists
	var email models.UniboxEmail
	if err := uc.db.Where("id = ? AND user_id = ?", emailID, user.ID).First(&email).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Email not found",
		})
	}

	updates := make(map[string]interface{})
	if req.IsRead != nil {
		updates["is_read"] = *req.IsRead
	}
	if req.IsStarred != nil {
		updates["is_starred"] = *req.IsStarred
	}
	if req.IsImportant != nil {
		updates["is_important"] = *req.IsImportant
	}

	if len(updates) > 0 {
		if err := uc.db.Model(&email).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update email",
			})
		}
	}

	return c.JSON(fiber.Map{
		"message": "Email updated successfully",
	})
}

// CreateFolder creates a new folder
func (uc *UniboxController) CreateFolder(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var req struct {
		Name  string `json:"name" validate:"required"`
		Icon  string `json:"icon"`
		Color string `json:"color"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check if folder already exists
	var existingFolder models.UniboxFolder
	if err := uc.db.Where("user_id = ? AND name = ?", user.ID, req.Name).First(&existingFolder).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Folder already exists",
		})
	}

	folder := models.UniboxFolder{
		UserID: user.ID,
		Name:   req.Name,
		Icon:   req.Icon,
		Color:  req.Color,
	}

	if err := uc.db.Create(&folder).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create folder",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(folder)
}

// GetFolders returns all folders for the user
func (uc *UniboxController) GetFolders(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var folders []models.UniboxFolder
	if err := uc.db.Where("user_id = ?", user.ID).Find(&folders).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch folders",
		})
	}

	return c.JSON(folders)
}

// DeleteFolder deletes a folder
func (uc *UniboxController) DeleteFolder(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	folderID := c.Params("id")

	// Check if folder exists and is not a system folder
	var folder models.UniboxFolder
	if err := uc.db.Where("id = ? AND user_id = ?", folderID, user.ID).First(&folder).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Folder not found",
		})
	}

	if folder.System {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "System folders cannot be deleted",
		})
	}

	// First remove all email-folder associations
	if err := uc.db.Where("folder_id = ?", folder.ID).Delete(&models.UniboxEmailFolder{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove email associations",
		})
	}

	// Then delete the folder
	if err := uc.db.Delete(&folder).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete folder",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
