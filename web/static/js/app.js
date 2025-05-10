// JavaScript для главной страницы калькулятора
// Переменные currentToken и currentUser уже определены в common.js

// DOM элементы
const loginForm = document.getElementById('login-form');
const registerForm = document.getElementById('register-form');
const authContainer = document.getElementById('auth-container');
const calculator = document.getElementById('calculator');
const expressionsList = document.getElementById('expressions-list');
const userInfo = document.getElementById('user-info');
const userName = document.getElementById('user-name');
const logoutBtn = document.getElementById('logout-btn');
const loginTab = document.querySelector('.auth-tab[data-tab="login"]');
const registerTab = document.querySelector('.auth-tab[data-tab="register"]');
const loginTabContent = document.getElementById('login-tab');
const registerTabContent = document.getElementById('register-tab');
const expressionInput = document.getElementById('expression');
const calculateBtn = document.getElementById('calculate-btn');
const resultContainer = document.getElementById('result-container');
const calculationResult = document.getElementById('calculation-result');
const expressionStatus = document.getElementById('expression-status');
const expressionsBody = document.querySelector('#expressions-table tbody');
const authTabs = document.querySelectorAll('.auth-tab');
const authTabContents = document.querySelectorAll('.auth-tab-content');

// Инициализация приложения при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    // Проверяем авторизацию сразу при загрузке
    if (!currentToken && (window.location.pathname === '/' || window.location.pathname === '/index.html')) {
        window.location.href = '/auth';
        return;
    }
    
    initApp();
    initTabs();
    initEventHandlers();
});

// Универсальная функция для отправки запросов к API
async function sendAPIRequest(url, options = {}) {
    try {
        // Добавляем токен авторизации, если он есть
        if (currentToken && !options.headers) {
            options.headers = {
                'Authorization': `Bearer ${currentToken}`,
                'Content-Type': 'application/json'
            };
        } else if (currentToken && options.headers) {
            options.headers = {
                ...options.headers,
                'Authorization': `Bearer ${currentToken}`
            };
        } else if (!options.headers) {
            options.headers = {
                'Content-Type': 'application/json'
            };
        }
        
        // Добавляем credentials для использования кук
        options.credentials = 'include';
        
        // Отправляем запрос
        const response = await fetch(url, options);
        
        // Проверяем, что запрос успешен
        if (!response.ok) {
            if (response.status === 401) {
                // Если получили 401, значит токен устарел или недействителен
                currentToken = null;
                localStorage.removeItem('token');
                localStorage.removeItem('username');
                updateAuthDisplay();
                throw new Error('Ошибка авторизации. Пожалуйста, войдите снова.');
            }
            throw new Error(`Ошибка сервера: ${response.status}`);
        }
        
        // Получаем ответ как текст, чтобы проверить его
        const responseText = await response.text();
        
        // Если ответ пустой, возвращаем null
        if (!responseText) return null;
        
        // Особая обработка для случая, когда в ответе несколько JSON объектов
        // Проверяем, содержит ли ответ несколько JSON-объектов (например, два объекта один за другим)
        if (responseText.includes('}\n{') || responseText.includes('}{')) {
            console.log('Обнаружено несколько JSON объектов, берем последний');
            
            // Находим последний JSON объект в ответе
            const lastJsonStartIndex = responseText.lastIndexOf('{');
            const lastJsonEndIndex = responseText.lastIndexOf('}') + 1;
            
            if (lastJsonStartIndex >= 0 && lastJsonEndIndex > lastJsonStartIndex) {
                const lastJsonPart = responseText.substring(lastJsonStartIndex, lastJsonEndIndex);
                try {
                    return JSON.parse(lastJsonPart);
                } catch (e) {
                    console.error('Ошибка парсинга последнего JSON объекта:', e);
                }
            }
            
            // Если не удалось получить последний объект, пробуем первый
            const firstJsonStartIndex = responseText.indexOf('{');
            const firstJsonEndIndex = responseText.indexOf('}') + 1;
            
            if (firstJsonStartIndex >= 0 && firstJsonEndIndex > firstJsonStartIndex) {
                const firstJsonPart = responseText.substring(firstJsonStartIndex, firstJsonEndIndex);
                try {
                    return JSON.parse(firstJsonPart);
                } catch (e) {
                    console.error('Ошибка парсинга первого JSON объекта:', e);
                }
            }
        }
        
        // Стандартная обработка для одиночного JSON
        try {
            return JSON.parse(responseText);
        } catch (e) {
            console.error('Ошибка парсинга JSON:', e, 'Ответ:', responseText);
            
            // Если не получилось, пробуем очистить от BOM и других возможных проблемных символов
            let cleanText = responseText.replace(/^\ufeff/g, '').trim(); // Удаляем BOM если он есть
            
            // Пробуем найти начало и конец JSON объекта
            const startIndex = cleanText.indexOf('{');
            const endIndex = cleanText.indexOf('}') + 1;
            
            if (startIndex >= 0 && endIndex > startIndex) {
                const jsonPart = cleanText.substring(startIndex, endIndex);
                try {
                    return JSON.parse(jsonPart);
                } catch (e2) {
                    console.error('Повторная ошибка парсинга JSON:', e2);
                }
            }
            
            throw new Error(`Ошибка при разборе ответа сервера: ${e.message}`);
        }
    } catch (error) {
        console.error('Ошибка запроса:', error);
        throw error;
    }
}

// Функция инициализации приложения
function initApp() {
    // Устанавливаем имя пользователя если есть
    if (currentUser) {
        userName.textContent = currentUser;
    }
    
    // Проверяем авторизацию
    checkAuth();
    
    // Если пользователь авторизован, обновляем интерфейс
    updateAuthDisplay();
}

// Функция для инициализации обработчиков событий
function initEventHandlers() {
    // Обработчики форм
    if (loginForm) {
        loginForm.addEventListener('submit', handleLogin);
    }
    
    if (registerForm) {
        registerForm.addEventListener('submit', handleRegister);
    }
    
    // Обработчик выхода
    if (logoutBtn) {
        logoutBtn.addEventListener('click', handleLogout);
    }
    
    // Обработчик вычисления
    if (calculateBtn) {
        calculateBtn.addEventListener('click', handleCalculate);
    }
}

// Инициализация вкладок
function initTabs() {
    // Вкладки авторизации
    if (loginTab && registerTab) {
        loginTab.addEventListener('click', function() {
            loginTabContent.style.display = 'block';
            registerTabContent.style.display = 'none';
            loginTab.classList.add('active');
            registerTab.classList.remove('active');
        });
        
        registerTab.addEventListener('click', function() {
            registerTabContent.style.display = 'block';
            loginTabContent.style.display = 'none';
            registerTab.classList.add('active');
            loginTab.classList.remove('active');
        });
    }
}

// Проверка авторизации
async function checkAuth() {
    try {
        if (currentToken) {
            showMainContent();
            return true;
        }
        
        // Если мы на главной странице и пользователь не авторизован,
        // перенаправляем на страницу авторизации
        if (window.location.pathname === '/' || window.location.pathname === '/index.html') {
            window.location.href = '/auth';
        }
        return false;
    } catch (error) {
        console.error('Ошибка при проверке авторизации:', error);
        throw error;
    }
}

// Функция обновления отображения в зависимости от авторизации
function updateAuthDisplay() {
    if (currentToken) {
        // Пользователь авторизован
        if (authContainer) authContainer.style.display = 'none';
        if (calculator) calculator.style.display = 'block';
        if (expressionsList) expressionsList.style.display = 'block';
        if (userInfo) userInfo.style.display = 'flex';
        
        // Обновляем имя пользователя
        if (userName) userName.textContent = currentUser;
        
        // Загружаем историю выражений
        loadExpressions();
    } else {
        // Пользователь не авторизован
        if (authContainer) authContainer.style.display = 'flex';
        if (calculator) calculator.style.display = 'none';
        if (expressionsList) expressionsList.style.display = 'none';
        if (userInfo) userInfo.style.display = 'none';
    }
}

// Показать форму авторизации
function showLoginForm() {
    if (authContainer) {
        authContainer.style.display = 'flex';
    }
    
    if (calculator) {
        calculator.style.display = 'none';
    }
    
    if (expressionsList) {
        expressionsList.style.display = 'none';
    }
    
    if (userInfo) {
        userInfo.style.display = 'none';
    }
}

// Показать основной контент
function showMainContent() {
    if (authContainer) {
        authContainer.style.display = 'none';
    }
    
    if (calculator) {
        calculator.style.display = 'block';
    }
    
    if (expressionsList) {
        expressionsList.style.display = 'block';
    }
    
    if (userInfo) {
        userInfo.style.display = 'flex';
    }
}

// Обработчик входа
async function handleLogin(e) {
    e.preventDefault();
    
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;
    
    if (!username || !password) {
        alert('Пожалуйста, заполните все поля');
        return;
    }
    
    try {
        const data = await sendAPIRequest('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, password })
        });
        
        // Сохраняем токен
        if (data && data.token) {
            currentToken = data.token;
            currentUser = username;
            localStorage.setItem('token', currentToken);
            localStorage.setItem('username', username);
            
            // Показываем основной контент
            showMainContent();
            
            // Обновляем отображение в зависимости от авторизации
            updateAuthDisplay();
            
            // Загружаем выражения
            loadExpressions();
        } else {
            throw new Error('Токен не получен');
        }
    } catch (error) {
        console.error('Ошибка при попытке входа:', error);
        alert('Ошибка при попытке входа: ' + error.message);
    }
}

// Обработчик регистрации
async function handleRegister(e) {
    e.preventDefault();
    
    const username = document.getElementById('reg-username').value;
    const password = document.getElementById('reg-password').value;
    const passwordConfirm = document.getElementById('reg-password-confirm').value;
    
    if (!username || !password || !passwordConfirm) {
        alert('Пожалуйста, заполните все поля');
        return;
    }
    
    if (password !== passwordConfirm) {
        alert('Пароли не совпадают');
        return;
    }
    
    try {
        const data = await sendAPIRequest('/api/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, password })
        });
        
        alert('Регистрация успешна! Теперь вы можете войти.');
        
        // Переключаемся на вкладку входа
        if (loginTab) {
            loginTab.click();
        }
    } catch (error) {
        console.error('Ошибка при регистрации:', error);
        alert('Ошибка при регистрации: ' + error.message);
    }
}

// Обработчик выхода
function handleLogout() {
    // Удаляем токен
    currentToken = null;
    currentUser = null;
    localStorage.removeItem('token');
    localStorage.removeItem('username');
    
    // Обновляем отображение
    updateAuthDisplay();
    
    // Показываем форму авторизации
    showLoginForm();
}

// Обработчик отправки выражения на вычисление
async function handleCalculate() {
    // Проверка наличия выражения
    if (!expressionInput) {
        alert('Ошибка: не найден элемент ввода');
        return;
    }
    
    const expression = expressionInput.value.trim();
    if (!expression) {
        alert('Пожалуйста, введите выражение');
        return;
    }
    
    // Показываем блок результатов
    if (resultContainer) {
        resultContainer.style.display = 'block';
    }
    
    if (calculationResult) {
        calculationResult.textContent = 'Вычисление...';
    }
    
    if (expressionStatus) {
        expressionStatus.textContent = 'Статус: Отправка выражения...';
    }
    
    // Проверяем наличие токена
    if (!currentToken) {
        alert('Вы не авторизованы. Пожалуйста, войдите в систему.');
        showLoginForm();
        return;
    }
    
    try {
        // Отправляем запрос на вычисление
        const data = await sendAPIRequest('/api/calculate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${currentToken}`
            },
            body: JSON.stringify({ expression })
        });
        
        if (data && data.id) {
            if (expressionStatus) {
                expressionStatus.textContent = `Статус: Выражение отправлено на вычисление (ID: ${data.id})`;
            }
            
            // Запускаем проверку статуса
            setTimeout(() => checkExpressionStatus(data.id), 1000);
        } else {
            throw new Error('Не удалось получить ID выражения');
        }
        
        // Обновляем список выражений
        setTimeout(loadExpressions, 2000);
        
    } catch (error) {
        console.error('Ошибка при отправке выражения:', error);
        if (expressionStatus) {
            expressionStatus.textContent = `Статус: Ошибка`;
        }
        if (calculationResult) {
            calculationResult.textContent = `Ошибка: ${error.message}`;
        }
    }
}

// Функция проверки статуса выражения
async function checkExpressionStatus(expressionId) {
    if (!expressionId) return;
    
    if (expressionStatus) {
        expressionStatus.textContent = `Статус: Проверка статуса...`;
    }
    
    try {
        const data = await sendAPIRequest(`/api/expression/${expressionId}`);
        
        if (data) {
            const status = data.status || 'unknown';
            const result = data.result !== undefined ? data.result : 'Вычисляется...';
            
            if (expressionStatus) {
                expressionStatus.textContent = `Статус: ${status}`;
            }
            
            if (calculationResult) {
                calculationResult.textContent = result;
            }
            
            // Если статус все еще processing или pending, проверяем еще раз через 2 секунды
            if (status === 'processing' || status === 'pending') {
                setTimeout(() => checkExpressionStatus(expressionId), 2000);
            } else if (status === 'completed') {
                // Обновляем список выражений
                loadExpressions();
            }
        } else {
            throw new Error('Не удалось получить информацию о выражении');
        }
    } catch (error) {
        console.error('Ошибка при проверке статуса:', error);
        if (expressionStatus) {
            expressionStatus.textContent = `Статус: Ошибка получения статуса`;
        }
    }
}

// Загрузка списка выражений
async function loadExpressions() {
    if (!expressionsBody) return;
    
    // Проверяем, авторизован ли пользователь
    if (!currentToken) {
        expressionsBody.innerHTML = '<tr><td colspan="4">Для просмотра истории необходимо авторизоваться</td></tr>';
        return;
    }
    
    // Очищаем таблицу перед загрузкой
    expressionsBody.innerHTML = '<tr><td colspan="4">Загрузка данных...</td></tr>';
    
    try {
        // Пробуем получить выражения для конкретного пользователя из базы данных
        let response;
        try {
            // Убедимся, что токен действительно есть
            const token = localStorage.getItem('token');
            if (!token) {
                throw new Error('Нет токена авторизации');
            }
            
            // Делаем запрос с актуальным токеном
            response = await fetch('/api/expressions', {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                }
            });
        } catch (networkError) {
            throw new Error(`Ошибка сети: ${networkError.message}`);
        }
        
        // Обрабатываем ответ сервера
        if (!response.ok) {
            if (response.status === 401 || response.status === 403) {
                expressionsBody.innerHTML = '<tr><td colspan="4">Необходимо авторизоваться для просмотра истории</td></tr>';
                // Возможно, токен устарел - перенаправляем на страницу авторизации
                setTimeout(() => {
                    window.location.href = '/auth';
                }, 2000);
                return;
            } else if (response.status === 500) {
                throw new Error('Внутренняя ошибка сервера');
            } else {
                throw new Error(`Ошибка сервера: ${response.status}`);
            }
        }
        
        let data;
        try {
            data = await response.json();
        } catch (jsonError) {
            throw new Error(`Ошибка парсинга JSON: ${jsonError.message}`);
        }
        
        // Очищаем таблицу
        expressionsBody.innerHTML = '';
        
        if (data && Array.isArray(data.expressions)) {
            // Если выражений нет
            if (data.expressions.length === 0) {
                const row = document.createElement('tr');
                row.innerHTML = '<td colspan="4">Нет вычисленных выражений</td>';
                expressionsBody.appendChild(row);
                return;
            }
            
            try {
                // Сортируем выражения по дате (сначала новые)
                data.expressions.sort((a, b) => {
                    try {
                        return new Date(b.created_at || 0) - new Date(a.created_at || 0);
                    } catch (e) {
                        return 0; // В случае ошибки при парсинге даты
                    }
                });
                
                // Добавляем выражения в таблицу
                data.expressions.forEach(expr => {
                    const row = document.createElement('tr');
                    row.innerHTML = `
                        <td><a href="/expression/${expr.id || ''}">${expr.id || 'N/A'}</a></td>
                        <td>${expr.expression || 'N/A'}</td>
                        <td class="${getStatusClass(expr.status)}">${expr.status || 'unknown'}</td>
                        <td>${expr.result !== null && expr.result !== undefined ? expr.result : 'N/A'}</td>
                    `;
                    expressionsBody.appendChild(row);
                });
            } catch (renderError) {
                console.error('Ошибка при отображении данных:', renderError);
                expressionsBody.innerHTML = `<tr><td colspan="4">Ошибка при отображении данных</td></tr>`;
            }
        } else {
            expressionsBody.innerHTML = '<tr><td colspan="4">Данные получены в неожиданном формате</td></tr>';
        }
    } catch (error) {
        console.error('Ошибка при загрузке списка выражений:', error);
        expressionsBody.innerHTML = `<tr><td colspan="4">Ошибка загрузки выражений: ${error.message}</td></tr>`;
    }
}

// Функция для определения класса статуса
function getStatusClass(status) {
    switch (status) {
        case 'pending':
            return 'status-pending';
        case 'processing':
            return 'status-processing';
        case 'completed':
            return 'status-completed';
        case 'error':
            return 'status-error';
        default:
            return '';
    }
}

// Инициализация при загрузке страницы
updateAuthDisplay();
