package handlers

import (
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// TemplateRenderer is a custom html/template renderer for Echo framework
type TemplateRenderer struct {
	templates *template.Template
}

// NewTemplateRenderer creates a new template renderer
func NewTemplateRenderer(templatesDir string) *TemplateRenderer {
	templates := template.New("")
	template.Must(templates.ParseGlob(templatesDir + "/auth/*.html"))
	template.Must(templates.ParseGlob(templatesDir + "/dashboard/*.html"))
	return &TemplateRenderer{templates: templates}
}

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// PageData holds common page data
type PageData struct {
	Title string
	User  interface{}
}

// LoginPage renders the login page
func (h *Handler) LoginPage(c echo.Context) error {
	return c.Render(http.StatusOK, "login.html", PageData{Title: "Login"})
}

// RegisterPage renders the register page
func (h *Handler) RegisterPage(c echo.Context) error {
	return c.Render(http.StatusOK, "register.html", PageData{Title: "Register"})
}

// DashboardPage renders the dashboard page
func (h *Handler) DashboardPage(c echo.Context) error {
	return c.Render(http.StatusOK, "index.html", PageData{Title: "Dashboard"})
}

// ProvidersPage renders the providers configuration page
func (h *Handler) ProvidersPage(c echo.Context) error {
	return c.Render(http.StatusOK, "providers.html", PageData{Title: "Service Configuration"})
}

// LogoutPage handles logout and redirects to login
func (h *Handler) LogoutPage(c echo.Context) error {
	return c.Redirect(http.StatusFound, "/login")
}
