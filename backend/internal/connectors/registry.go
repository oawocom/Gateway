package connectors

// ConfigField describes one input field a connector needs.
type ConfigField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"` // text | password | url
	Required bool   `json:"required"`
	Hint     string `json:"hint,omitempty"`
}

// Descriptor is a catalog entry shown in the marketplace.
type Descriptor struct {
	Type        string        `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	Available   bool          `json:"available"`
	Hidden      bool          `json:"hidden,omitempty"` // not shown as a card; fields still used by wizards
	Fields      []ConfigField `json:"fields,omitempty"`
}

var Catalog = []Descriptor{
	{
		Type:        "1c",
		Name:        "1C",
		Description: "1C bazanızı Gateway-ə qoşun: versiyanı seçin, sistem qoşulma üsulunu özü təyin etsin. Hesabatlar qoşulmadan sonra avtomatik açılır.",
		Category:    "ERP / Mühasibat",
		Available:   true,
	},
	{
		Type:        "1c_odata",
		Name:        "1C (OData, v8.3.5+)",
		Description: "Müasir 1C (8.3.5 və yuxarı) üçün standart OData interfeysi. Kataloqlar, sənədlər və registrlərə tam çıxış.",
		Category:    "ERP / Mühasibat",
		Available:   true,
		Hidden:      true,
		Fields: []ConfigField{
			{Key: "base_url", Label: "Publikasiya URL", Type: "url", Required: true, Hint: "məs. http://server/base (odata yolu avtomatik əlavə olunur)"},
			{Key: "username", Label: "İstifadəçi adı", Type: "text", Required: true},
			{Key: "password", Label: "Şifrə", Type: "password", Required: true},
		},
	},
	{
		Type:        "1c_http",
		Name:        "1C HTTP Servis (bütün versiyalar)",
		Description: "OData olmayan və ya köhnə 1C üçün: bazada publikasiya olunmuş xüsusi HTTP servislərdən data çəkir.",
		Category:    "ERP / Mühasibat",
		Available:   true,
		Hidden:      true,
		Fields: []ConfigField{
			{Key: "base_url", Label: "Publikasiya URL", Type: "url", Required: true, Hint: "məs. http://server/base"},
			{Key: "username", Label: "İstifadəçi adı", Type: "text", Required: true},
			{Key: "password", Label: "Şifrə", Type: "password", Required: true},
			{Key: "endpoints", Label: "Endpointlər", Type: "textarea", Required: true, Hint: "Hər sətirdə: Ad:/hs/yol — məs. Müştərilər:/hs/api/customers"},
		},
	},
	{
		Type:        "postgres",
		Name:        "PostgreSQL",
		Description: "PostgreSQL bazasına birbaşa qoşulun — cədvəlləri seçib sinxronlaşdırın (1C PostgreSQL üstündədirsə də işləyir).",
		Category:    "Verilənlər bazası",
		Available:   true,
		Fields:      sqlFields(true),
	},
	{
		Type:        "mysql",
		Name:        "MySQL / MariaDB",
		Description: "MySQL və ya MariaDB bazasına birbaşa qoşulun və cədvəlləri sinxronlaşdırın.",
		Category:    "Verilənlər bazası",
		Available:   true,
		Fields:      sqlFields(false),
	},
	{
		Type:        "mssql",
		Name:        "Microsoft SQL Server",
		Description: "MSSQL bazasına birbaşa qoşulun (1C çox vaxt MSSQL üstündə işləyir) və cədvəlləri sinxronlaşdırın.",
		Category:    "Verilənlər bazası",
		Available:   true,
		Fields:      sqlFields(false),
	},
	{
		Type:        "zoho_crm",
		Name:        "Zoho CRM",
		Description: "Zoho CRM modullarını (Leads, Contacts, Deals və s.) Gateway-ə sinxronlaşdırın. Self Client məlumatları ilə qoşulur.",
		Category:    "CRM",
		Available:   true,
		Fields: []ConfigField{
			{Key: "region", Label: "Region", Type: "text", Required: false, Hint: "com (standart) / eu / in / com.au / jp"},
			{Key: "client_id", Label: "Client ID", Type: "text", Required: true, Hint: "api-console.zoho.com → Self Client"},
			{Key: "client_secret", Label: "Client Secret", Type: "password", Required: true},
			{Key: "refresh_token", Label: "Refresh Token", Type: "password", Required: true, Hint: "Self Client grant kodundan alınmış refresh token"},
		},
	},
	{
		Type:        "pasha_bank",
		Name:        "PASHA Bank",
		Description: "PASHA Bank Open Banking API — biznes hesab qalıqları və çıxarışlar birbaşa Gateway-də. Token bank tərəfindən verilir.",
		Category:    "Bank",
		Available:   true,
		Fields: []ConfigField{
			{Key: "base_url", Label: "API mühiti (URL)", Type: "url", Required: false, Hint: "boş = canlı mühit; sandbox üçün: https://sandbox.pashabank.digital"},
			{Key: "token", Label: "API Token", Type: "password", Required: true, Hint: "PASHA Bank B2B inteqrasiya müqaviləsi ilə verilən token"},
		},
	},
	{
		Type:        "kapital_bank",
		Name:        "Kapital Bank (Birbank Biznes)",
		Description: "Kapital Bank (Birbank Biznes) açıq API — hesab qalıqları və çıxarışlar Gateway-də. Token bank portalından verilir.",
		Category:    "Bank",
		Available:   true,
		Fields: []ConfigField{
			{Key: "base_url", Label: "API URL", Type: "url", Required: false, Hint: "boş = https://my.birbank.business/b2b/api/public/v1"},
			{Key: "token", Label: "API Token", Type: "password", Required: true, Hint: "api.birbank.business portalından alınan token"},
		},
	},
	{
		Type:        "epoint",
		Name:        "EPoint",
		Description: "EPoint ödəniş sistemi — açarlarla qoşulma və yoxlama. Əməliyyat sinxronizasiyası tezliklə aktivləşir.",
		Category:    "Ödəniş sistemləri",
		Available:   true,
		Fields: []ConfigField{
			{Key: "public_key", Label: "Public Key", Type: "text", Required: true},
			{Key: "private_key", Label: "Private Key", Type: "password", Required: true},
		},
	},
	{
		Type:        "yigim",
		Name:        "YIĞIM",
		Description: "YIĞIM ödəniş aqreqatoru — merchant qoşulması. Ödəniş sinxronizasiyası tezliklə aktivləşir.",
		Category:    "Ödəniş sistemləri",
		Available:   true,
		Fields: []ConfigField{
			{Key: "base_url", Label: "API URL", Type: "url", Required: false, Hint: "boş = https://api.yigim.az"},
			{Key: "merchant", Label: "Merchant kodu", Type: "text", Required: true},
			{Key: "secret_key", Label: "Secret Key", Type: "password", Required: true},
		},
	},
}

func Get(connType string) *Descriptor {
	for i := range Catalog {
		if Catalog[i].Type == connType {
			return &Catalog[i]
		}
	}
	return nil
}

// sqlFields returns the shared field set for SQL database connectors.
func sqlFields(withSSL bool) []ConfigField {
	f := []ConfigField{
		{Key: "host", Label: "Host", Type: "text", Required: true, Hint: "məs. db.sirket.az və ya IP"},
		{Key: "port", Label: "Port", Type: "text", Required: false, Hint: "boş = standart port"},
		{Key: "database", Label: "Baza adı", Type: "text", Required: true},
		{Key: "username", Label: "İstifadəçi adı", Type: "text", Required: true},
		{Key: "password", Label: "Şifrə", Type: "password", Required: true},
	}
	if withSSL {
		f = append(f, ConfigField{Key: "sslmode", Label: "SSL rejimi", Type: "text", Required: false, Hint: "prefer (standart) / require / disable"})
	}
	return f
}
