document.addEventListener('DOMContentLoaded', function() {
    // Переключение между вкладками входа и регистрации
    const tabs = document.querySelectorAll('.auth-tab');
    const tabContents = document.querySelectorAll('.auth-tab-content');
    
    tabs.forEach(tab => {
        tab.addEventListener('click', function() {
            // Удаляем активный класс со всех вкладок
            tabs.forEach(t => t.classList.remove('active'));
            
            // Добавляем активный класс на нажатую вкладку
            this.classList.add('active');
            
            // Скрываем все содержимое вкладок
            tabContents.forEach(content => content.classList.remove('active'));
            
            // Показываем содержимое выбранной вкладки
            const tabId = this.dataset.tab;
            document.getElementById(`${tabId}-tab`).classList.add('active');
        });
    });
    
    // Обработка формы входа
    const loginForm = document.getElementById('login-form');
    if (loginForm) {
        loginForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const username = document.getElementById('login-username').value;
            const password = document.getElementById('login-password').value;
            
            if (!username || !password) {
                alert('Пожалуйста, введите логин и пароль');
                return;
            }
            
            try {
                const response = await fetch('/api/login', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({ username, password })
                });
                
                if (!response.ok) {
                    throw new Error('Ошибка авторизации');
                }
                
                const data = await response.json();
                currentToken = data.token;
                currentUser = username;
                
                localStorage.setItem('token', currentToken);
                localStorage.setItem('username', currentUser);
                
                window.location.href = '/';
            } catch (error) {
                console.error('Ошибка при входе:', error);
                alert('Ошибка при входе: ' + error.message);
            }
        });
    }
    
    // Обработка формы регистрации
    const registerForm = document.getElementById('register-form');
    if (registerForm) {
        registerForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const username = document.getElementById('register-username').value;
            const password = document.getElementById('register-password').value;
            const passwordConfirm = document.getElementById('register-password-confirm').value;
            
            if (!username || !password) {
                alert('Пожалуйста, введите логин и пароль');
                return;
            }
            
            if (password !== passwordConfirm) {
                alert('Пароли не совпадают');
                return;
            }
            
            try {
                const response = await fetch('/api/register', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({ username, password })
                });
                
                if (!response.ok) {
                    throw new Error('Ошибка регистрации');
                }
                
                alert('Регистрация успешна. Теперь вы можете войти.');
                
                // Переключаемся на вкладку входа
                tabs.forEach(t => t.classList.remove('active'));
                document.querySelector('[data-tab="login"]').classList.add('active');
                
                tabContents.forEach(content => content.classList.remove('active'));
                document.getElementById('login-tab').classList.add('active');
                
                // Заполняем поля формы входа
                document.getElementById('login-username').value = username;
                document.getElementById('login-password').value = '';
            } catch (error) {
                console.error('Ошибка при регистрации:', error);
                alert('Ошибка при регистрации: ' + error.message);
            }
        });
    }
});
