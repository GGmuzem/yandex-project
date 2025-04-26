package orchestrator

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

// WebHandler обработчик для веб-интерфейса
type WebHandler struct {
	StaticDir   string
	TemplateDir string
	templates   map[string]*template.Template
}

// NewWebHandler создает новый обработчик для веб-интерфейса
func NewWebHandler(staticDir, templateDir string) *WebHandler {
	handler := &WebHandler{
		StaticDir:   staticDir,
		TemplateDir: templateDir,
		templates:   make(map[string]*template.Template),
	}

	// Загружаем шаблоны
	handler.loadTemplates()

	return handler
}

// loadTemplates загружает шаблоны из директории
func (h *WebHandler) loadTemplates() {
	layouts, err := filepath.Glob(filepath.Join(h.TemplateDir, "layouts", "*.html"))
	if err != nil {
		log.Printf("Ошибка при загрузке шаблонов: %v", err)
		return
	}

	includes, err := filepath.Glob(filepath.Join(h.TemplateDir, "includes", "*.html"))
	if err != nil {
		log.Printf("Ошибка при загрузке включаемых файлов: %v", err)
		return
	}

	// Загружаем все шаблоны страниц
	pages, err := filepath.Glob(filepath.Join(h.TemplateDir, "pages", "*.html"))
	if err != nil {
		log.Printf("Ошибка при загрузке страниц: %v", err)
		return
	}

	// Создаем шаблоны для каждой страницы
	for _, page := range pages {
		files := append(layouts, includes...)
		files = append(files, page)

		name := filepath.Base(page)
		tmpl, err := template.ParseFiles(files...)
		if err != nil {
			log.Printf("Ошибка при парсинге шаблона %s: %v", name, err)
			continue
		}

		h.templates[name] = tmpl
	}

	log.Printf("Загружено %d шаблонов", len(h.templates))
}

// ServeStaticFiles обрабатывает запросы к статическим файлам
func (h *WebHandler) ServeStaticFiles() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.Dir(h.StaticDir)))
}

// RenderTemplate отображает шаблон
func (h *WebHandler) RenderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := h.templates[name]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	err := tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// IndexHandler обработчик главной страницы
func (h *WebHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	h.RenderTemplate(w, "index.html", nil)
}

// SetupWebRoutes настраивает маршруты для веб-интерфейса
func (h *WebHandler) SetupWebRoutes() {
	// Статические файлы
	http.Handle("/static/", h.ServeStaticFiles())

	// Главная страница
	http.HandleFunc("/", h.IndexHandler)

	// Страница регистрации
	http.HandleFunc("/register", h.RegisterPageHandler)

	// Страница входа
	http.HandleFunc("/login", h.LoginPageHandler)

	// Страница калькулятора
	http.HandleFunc("/calculator", h.CalculatorPageHandler)
}

// RegisterPageHandler обработчик страницы регистрации
func (h *WebHandler) RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
	h.RenderTemplate(w, "register.html", nil)
}

// LoginPageHandler обработчик страницы входа
func (h *WebHandler) LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	h.RenderTemplate(w, "login.html", nil)
}

// CalculatorPageHandler обработчик страницы калькулятора
func (h *WebHandler) CalculatorPageHandler(w http.ResponseWriter, r *http.Request) {
	h.RenderTemplate(w, "calculator.html", nil)
}
