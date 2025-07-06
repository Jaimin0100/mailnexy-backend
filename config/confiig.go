package config

import (
	"fmt"
	"log"
	"mailnexy/models"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB        *gorm.DB
	AppConfig Config
	envLoaded bool
)

type RedisConfig struct {
	Enabled  bool   `json:"enabled"`
	Address  string `json:"address"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

type OAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
}

type Config struct {
	Environment          string      `json:"environment"`
	Google               OAuthConfig `json:"google"`
	Microsoft            OAuthConfig `json:"microsoft"`
	Yahoo                OAuthConfig `json:"yahoo"`
	EncryptionKey        string      `json:"-"`
	ServerPort           string      `json:"server_port"`
	DBHost               string      `json:"db_host"`
	DBPort               string      `json:"db_port"`
	DBUser               string      `json:"db_user"`
	DBPassword           string      `json:"-"`
	DBName               string      `json:"db_name"`
	DBSSLMode            string      `json:"db_ssl_mode"`
	DBMaxIdleConns       int         `json:"db_max_idle_conns"`
	DBMaxOpenConns       int         `json:"db_max_open_conns"`
	StripeSecretKey      string      `json:"stripe_secret_key"`
	StripePublishableKey string      `json:"stripe_publishable_key"`
	StripeWebhookSecret  string      `json:"stripe_webhook_secret"`
	WarmupEmail          string      `json:"warmup_email"`
	RateLimitTestSender  int         `json:"rate_limit_test_sender"`
	Redis                RedisConfig `json:"redis"`
	SMTPHost             string      `json:"smtp_host"`
	SMTPPort             string      `json:"smtp_port"`
	SMTPUsername         string      `json:"smtp_username"`
	SMTPPassword         string      `json:"smtp_password"`
	FromEmail            string      `json:"from_email"`
}

func init() {
	// Try to load .env file, but don't fail if it doesn't exist
	_ = godotenv.Load()
	envLoaded = true
}

func LoadConfig() error {
	AppConfig = Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		Google: OAuthConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURI:  getEnv("GOOGLE_REDIRECT_URI", ""),
		},
		Microsoft: OAuthConfig{
			ClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
			ClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
			RedirectURI:  getEnv("MICROSOFT_REDIRECT_URI", ""),
		},
		Yahoo: OAuthConfig{
			ClientID:     getEnv("YAHOO_CLIENT_ID", ""),
			ClientSecret: getEnv("YAHOO_CLIENT_SECRET", ""),
			RedirectURI:  getEnv("YAHOO_REDIRECT_URI", ""),
		},
		EncryptionKey:  getEnv("ENCRYPTION_KEY", ""),
		ServerPort:     getEnv("SERVER_PORT", "5000"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", ""),
		DBName:         getEnv("DB_NAME", "mailnexy"),
		DBSSLMode:      getEnv("DB_SSL_MODE", "disable"),
		DBMaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
		DBMaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 100),

		StripeSecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
		StripePublishableKey: getEnv("STRIPE_PUBLISHABLE_KEY", ""),
		StripeWebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
		WarmupEmail:          getEnv("WARMUP_EMAIL_RECIPIENT", "default_warmup_target@example.com"), // <--- POPULATE IT
	}

	// Validate required configurations
	if AppConfig.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if AppConfig.EncryptionKey == "" {
		return fmt.Errorf("ENCRYPTION_KEY is required")
	}
	if AppConfig.StripeSecretKey == "" {
		return fmt.Errorf("STRIPE_SECRET_KEY is required for payment processing")
	}
	if AppConfig.Environment == "production" {
		if AppConfig.Google.ClientID == "" || AppConfig.Google.ClientSecret == "" {
			return fmt.Errorf("Google OAuth credentials are required in production")
		}
		// Add similar checks for other providers
	}

	logConfig()
	return nil
}

func ConnectDB() error {
	log.Println("Attempting to connect to database...")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		AppConfig.DBHost,
		AppConfig.DBPort,
		AppConfig.DBUser,
		AppConfig.DBPassword,
		AppConfig.DBName,
		AppConfig.DBSSLMode,
	)
	log.Println("Using connection string:", maskPassword(dsn))

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get DB instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(AppConfig.DBMaxIdleConns)
	sqlDB.SetMaxOpenConns(AppConfig.DBMaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	log.Println("✅ Successfully connected to the database")
	// Add migration logic here
	log.Println("🔄 Starting database migration...")
	if err := migrateDB(DB); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}
	log.Println("✅ Database migration completed")
	return nil
}

// Helper functions
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if !envLoaded && fallback == "" {
		log.Printf("⚠️ Environment variable %s not found and no fallback provided", key)
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return fallback
	}
	var value int
	_, err := fmt.Sscanf(valueStr, "%d", &value)
	if err != nil {
		return fallback
	}
	return value
}

func maskPassword(dsn string) string {
	const passwordMarker = "password="
	startIdx := strings.Index(dsn, passwordMarker)
	if startIdx == -1 {
		return dsn
	}

	startIdx += len(passwordMarker)
	endIdx := strings.IndexAny(dsn[startIdx:], " ")
	if endIdx == -1 {
		return dsn[:startIdx] + "*****"
	}
	return dsn[:startIdx] + "*****" + dsn[startIdx+endIdx:]
}

func logConfig() {
	log.Println("🔧 Loaded configuration:")
	log.Printf("Environment: %s", AppConfig.Environment)
	log.Printf("Server Port: %s", AppConfig.ServerPort)
	log.Printf("Database: %s@%s:%s/%s",
		AppConfig.DBUser,
		AppConfig.DBHost,
		AppConfig.DBPort,
		AppConfig.DBName)
	log.Printf("OAuth Providers: Google(%t), Microsoft(%t), Yahoo(%t)",
		AppConfig.Google.ClientID != "",
		AppConfig.Microsoft.ClientID != "",
		AppConfig.Yahoo.ClientID != "")
}

func migrateDB(db *gorm.DB) error {

	// Disable foreign key constraints during migration
	if err := db.Exec("SET CONSTRAINTS ALL DEFERRED").Error; err != nil {
		return fmt.Errorf("failed to defer constraints: %w", err)
	}

	// Get the database dialect
	dialect := db.Dialector.Name()

	// For PostgreSQL, drop constraints more carefully
	if dialect == "postgres" {
		// Check if constraint exists before trying to drop it
		if err := db.Exec(`
            DO $$
            BEGIN
                IF EXISTS (
                    SELECT 1 FROM pg_constraint 
                    WHERE conname = 'uni_users_email'
                ) THEN
                    EXECUTE 'ALTER TABLE users DROP CONSTRAINT uni_users_email';
                END IF;
            END $$;
        `).Error; err != nil {
			return fmt.Errorf("failed to conditionally drop constraint: %w", err)
		}
	}

	return db.AutoMigrate(
		&models.User{},
		&models.RefreshToken{},
		&models.Plan{},
		&models.CreditTransaction{},
		&models.CreditUsage{},
		&models.Sender{},
		&models.WarmupSchedule{},
		&models.WarmupStage{},
		&models.EmailTracking{},
		&models.Campaign{},
		&models.CampaignFlow{},
		&models.CampaignExecution{},
		&models.CampaignLeadList{},
		&models.LeadList{},
		&models.Lead{},
		&models.LeadListMembership{},
		&models.LeadTag{},
		&models.LeadCustomField{},
		&models.CampaignActivity{},
		&models.ClickEvent{},
		&models.LeadActivity{},
		&models.EmailVerification{},
		&models.VerificationResult{},
		&models.APIKey{},
		&models.Unsubscribe{},
		&models.Bounce{},
		&models.Template{},
		&models.Sequence{},
		&models.SequenceStep{},
		&models.Team{},
		&models.TeamMember{},
		&models.UserFeature{},
		&models.Feature{},
		&models.UniboxEmail{},
		&models.UniboxFolder{},
		&models.UniboxEmailFolder{},
	)
}
