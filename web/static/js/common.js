// Общие функции для всех страниц
const API_URL = '/api';
let currentToken = localStorage.getItem('token') || '';
let currentUser = localStorage.getItem('username') || '';

// Функция для обновления элементов интерфейса в зависимости от статуса авторизации
function updateAuthDisplay() {
    const userInfo = document.getElementById('user-info');
    const loginForm = document.getElementById('login-form');
    
    if (userInfo && loginForm) {
        if (currentToken) {
            if (loginForm) loginForm.style.display = 'none';
            if (userInfo) {
                userInfo.style.display = 'flex';
                const userName = document.getElementById('user-name');
                if (userName) userName.textContent = currentUser;
            }
        } else {
            if (loginForm) loginForm.style.display = 'flex';
            if (userInfo) userInfo.style.display = 'none';
        }
    }
}

// Функция логаута
function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('username');
    currentToken = '';
    currentUser = '';
    updateAuthDisplay();
    
    // Если мы на защищенной странице, перенаправляем на главную
    const protectedPages = ['/history', '/expression/'];
    const currentPath = window.location.pathname;
    
    for (const page of protectedPages) {
        if (currentPath.includes(page)) {
            window.location.href = '/';
            return;
        }
    }
}

// Инициализация общих элементов
document.addEventListener('DOMContentLoaded', function() {
    updateAuthDisplay();
    
    // Добавляем обработчик для кнопки выхода
    const logoutBtn = document.getElementById('logout-btn');
    if (logoutBtn) {
        logoutBtn.addEventListener('click', logout);
    }
});

// Вспомогательная функция для API-запросов с авторизацией
async function fetchWithAuth(url, options = {}) {
    if (!options.headers) {
        options.headers = {};
    }
    
    if (currentToken) {
        options.headers['Authorization'] = `Bearer ${currentToken}`;
    }
    
    options.headers['Content-Type'] = 'application/json';
    
    try {
        const response = await fetch(url, options);
        
        if (response.status === 401) {
            // Токен недействителен, выполняем логаут
            logout();
            throw new Error('Требуется авторизация');
        }
        
        return response;
    } catch (error) {
        console.error('Ошибка при выполнении запроса:', error);
        throw error;
    }
}
