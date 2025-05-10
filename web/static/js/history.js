// Переменные для пагинации
let currentPage = 1;
let totalPages = 1;
let pageSize = 10;
let expressions = [];

// Функция для загрузки истории выражений с сервера
async function loadExpressions() {
    try {
        const filters = getFilters();
        const offset = (currentPage - 1) * pageSize;
        
        const response = await fetchWithAuth(`/api/expressions?offset=${offset}&limit=${pageSize}${filters}`);
        
        if (!response.ok) {
            throw new Error('Не удалось загрузить историю вычислений');
        }
        
        const data = await response.json();
        expressions = data.expressions || [];
        totalPages = Math.ceil((data.total || expressions.length) / pageSize);
        
        renderHistory();
        updatePagination();
    } catch (error) {
        console.error('Ошибка при загрузке истории:', error);
        alert('Ошибка при загрузке истории: ' + error.message);
    }
}

// Получение строки с фильтрами из формы
function getFilters() {
    let filters = '';
    
    const dateFrom = document.getElementById('date-from').value;
    const dateTo = document.getElementById('date-to').value;
    const status = document.getElementById('status-filter').value;
    
    if (dateFrom) {
        filters += `&date_from=${dateFrom}`;
    }
    
    if (dateTo) {
        filters += `&date_to=${dateTo}`;
    }
    
    if (status && status !== 'all') {
        filters += `&status=${status}`;
    }
    
    return filters;
}

// Отображение истории вычислений в таблице
function renderHistory() {
    const tbody = document.getElementById('history-body');
    tbody.innerHTML = '';
    
    if (expressions.length === 0) {
        const row = document.createElement('tr');
        row.innerHTML = `<td colspan="6" class="empty-table">История вычислений пуста</td>`;
        tbody.appendChild(row);
        return;
    }
    
    expressions.forEach(expr => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${expr.id}</td>
            <td>${expr.expression}</td>
            <td>${formatDate(expr.created_at || new Date().toISOString())}</td>
            <td>${getStatusBadge(expr.status)}</td>
            <td>${expr.result !== undefined ? expr.result : '-'}</td>
            <td><a href="/expression/${expr.id}" class="btn btn-small">Детали</a></td>
        `;
        tbody.appendChild(row);
    });
}

// Форматирование даты
function formatDate(dateString) {
    const date = new Date(dateString);
    return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}

// Получение цветного статуса
function getStatusBadge(status) {
    let badgeClass = '';
    let statusText = '';
    
    switch (status) {
        case 'completed':
            badgeClass = 'badge-success';
            statusText = 'Завершено';
            break;
        case 'pending':
            badgeClass = 'badge-warning';
            statusText = 'В процессе';
            break;
        case 'error':
            badgeClass = 'badge-danger';
            statusText = 'Ошибка';
            break;
        default:
            badgeClass = 'badge-secondary';
            statusText = status || 'Неизвестно';
    }
    
    return `<span class="badge ${badgeClass}">${statusText}</span>`;
}

// Обновление пагинации
function updatePagination() {
    const pageInfo = document.getElementById('page-info');
    const prevBtn = document.getElementById('prev-page');
    const nextBtn = document.getElementById('next-page');
    
    pageInfo.textContent = `Страница ${currentPage} из ${totalPages || 1}`;
    
    prevBtn.disabled = currentPage <= 1;
    nextBtn.disabled = currentPage >= totalPages;
}

// Инициализация страницы истории
document.addEventListener('DOMContentLoaded', function() {
    // Проверяем авторизацию
    if (!currentToken) {
        window.location.href = '/auth';
        return;
    }
    
    // Загружаем историю
    loadExpressions();
    
    // Обработчик фильтрации
    const applyFilterBtn = document.getElementById('apply-filter');
    if (applyFilterBtn) {
        applyFilterBtn.addEventListener('click', function() {
            currentPage = 1; // Сбрасываем на первую страницу при фильтрации
            loadExpressions();
        });
    }
    
    // Обработчики пагинации
    const prevBtn = document.getElementById('prev-page');
    const nextBtn = document.getElementById('next-page');
    
    if (prevBtn) {
        prevBtn.addEventListener('click', function() {
            if (currentPage > 1) {
                currentPage--;
                loadExpressions();
            }
        });
    }
    
    if (nextBtn) {
        nextBtn.addEventListener('click', function() {
            if (currentPage < totalPages) {
                currentPage++;
                loadExpressions();
            }
        });
    }
});
