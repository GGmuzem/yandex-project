// Проверка авторизации при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    const token = localStorage.getItem('token');
    const logoutItem = document.getElementById('logout-item');
    
    // Если есть токен, скрываем ссылки на вход и регистрацию, показываем кнопку выхода
    if (token) {
        const navItems = document.querySelectorAll('.navbar-nav a');
        navItems.forEach(item => {
            if (item.getAttribute('href') === '/login' || item.getAttribute('href') === '/register') {
                item.parentElement.classList.add('d-none');
            }
        });
        
        if (logoutItem) {
            logoutItem.classList.remove('d-none');
        }
    } else {
        if (logoutItem) {
            logoutItem.classList.add('d-none');
        }
    }
    
    // Обработчик выхода
    const logoutLink = document.getElementById('logout-link');
    if (logoutLink) {
        logoutLink.addEventListener('click', function(e) {
            e.preventDefault();
            localStorage.removeItem('token');
            window.location.href = '/';
        });
    }
    
    // Подсветка активного пункта меню
    const currentPath = window.location.pathname;
    const navLinks = document.querySelectorAll('.navbar-nav a');
    
    navLinks.forEach(link => {
        if (link.getAttribute('href') === currentPath) {
            link.classList.add('active');
        }
    });
}); 