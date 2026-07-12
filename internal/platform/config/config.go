package config

import "github.com/kelseyhightower/envconfig"

// TicketAPIConfig holds environment-based configuration for the ticket API service.
type TicketAPIConfig struct {
	Port              int    `envconfig:"PORT" default:"8080"`
	DBHost            string `envconfig:"DB_HOST" default:"localhost"`
	DBPort            int    `envconfig:"DB_PORT" default:"3306"`
	DBUser            string `envconfig:"DB_USER" default:"root"`
	DBPassword        string `envconfig:"DB_PASSWORD" default:"root"`
	DBName            string `envconfig:"DB_NAME" default:"tickets_db"`
	RabbitMQURL       string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672/"`
	CognitoRegion     string `envconfig:"COGNITO_REGION" default:"us-east-1"`
	CognitoUserPoolID string `envconfig:"COGNITO_USER_POOL_ID"`
	CORSAllowedOrigin string `envconfig:"CORS_ALLOWED_ORIGIN"`
}

// ValidatorAPIConfig holds environment-based configuration for the validator API service.
type ValidatorAPIConfig struct {
	Port              int    `envconfig:"PORT" default:"8081"`
	RedisHost         string `envconfig:"REDIS_HOST" default:"localhost"`
	RedisPort         int    `envconfig:"REDIS_PORT" default:"6379"`
	RabbitMQURL       string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672/"`
	TicketServiceURL  string `envconfig:"TICKET_SERVICE_URL" default:"http://localhost:8080"`
	HMACSecret        string `envconfig:"HMAC_SECRET" default:"change-me-in-production"`
	CognitoRegion     string `envconfig:"COGNITO_REGION" default:"us-east-1"`
	CognitoUserPoolID string `envconfig:"COGNITO_USER_POOL_ID"`
}

// QRWorkerConfig holds environment-based configuration for the QR worker service.
type QRWorkerConfig struct {
	RabbitMQURL  string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672/"`
	SMTPHost     string `envconfig:"SMTP_HOST" default:"localhost"`
	SMTPPort     int    `envconfig:"SMTP_PORT" default:"1025"`
	SMTPFrom     string `envconfig:"SMTP_FROM" default:"tickets@entradasqr.local"`
	SMTPUser     string `envconfig:"SMTP_USER"`
	SMTPPassword string `envconfig:"SMTP_PASSWORD"`
	QRSize       int    `envconfig:"QR_SIZE" default:"256"`
	HMACSecret   string `envconfig:"HMAC_SECRET" default:"change-me-in-production"`
}

// LoadTicketAPIConfig reads configuration from environment variables for the ticket API.
func LoadTicketAPIConfig() (*TicketAPIConfig, error) {
	var cfg TicketAPIConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadQRWorkerConfig reads configuration from environment variables for the QR worker.
func LoadQRWorkerConfig() (*QRWorkerConfig, error) {
	var cfg QRWorkerConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadValidatorAPIConfig reads configuration from environment variables for the validator API.
func LoadValidatorAPIConfig() (*ValidatorAPIConfig, error) {
	var cfg ValidatorAPIConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
