package handlers

import (
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

type TemplateRenderer struct {
	templates *template.Template
}

func NewTemplateRenderer(templatesDir string) *TemplateRenderer {
	templates := template.New("")
	template.Must(templates.ParseGlob(templatesDir + "/auth/*.html"))
	template.Must(templates.ParseGlob(templatesDir + "/index.html"))
	template.Must(templates.ParseGlob(templatesDir + "/dashboard/*.html"))
	return &TemplateRenderer{templates: templates}
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

type PageData struct {
	Title string
	User  interface{}
}

func (h *Handler) IndexPage(c echo.Context) error {
	return c.Render(http.StatusOK, "index.html", PageData{Title: "AI Gateway"})
}

func (h *Handler) LoginPage(c echo.Context) error {
	return c.Render(http.StatusOK, "login.html", PageData{Title: "Login"})
}

func (h *Handler) RegisterPage(c echo.Context) error {
	return c.Render(http.StatusOK, "register.html", PageData{Title: "Register"})
}

func (h *Handler) DashboardPage(c echo.Context) error {
	return c.Render(http.StatusOK, "index.html", PageData{Title: "Dashboard"})
}

func (h *Handler) ProvidersPage(c echo.Context) error {
	return c.Render(http.StatusOK, "providers.html", PageData{Title: "Service Configuration"})
}

func (h *Handler) KeysPage(c echo.Context) error {
	return c.Render(http.StatusOK, "keys.html", PageData{Title: "API Keys"})
}

func (h *Handler) LogoutPage(c echo.Context) error {
	return c.Redirect(http.StatusFound, "/login")
}
