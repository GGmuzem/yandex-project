// Получение ID выражения из URL
function getExpressionId() {
    const pathParts = window.location.pathname.split('/');
    return pathParts[pathParts.length - 1];
}

// Загрузка деталей выражения
async function loadExpressionDetails() {
    try {
        const exprId = getExpressionId();
        const response = await fetchWithAuth(`/api/expression/${exprId}`);
        
        if (!response.ok) {
            throw new Error('Не удалось загрузить детали выражения');
        }
        
        const data = await response.json();
        renderExpressionDetails(data);
        loadExpressionTasks(exprId);
    } catch (error) {
        console.error('Ошибка при загрузке деталей выражения:', error);
        alert('Ошибка при загрузке деталей выражения: ' + error.message);
    }
}

// Отображение данных выражения
function renderExpressionDetails(expression) {
    document.getElementById('expr-id').textContent = expression.id;
    document.getElementById('expr-text').textContent = expression.expression;
    document.getElementById('expr-status').textContent = getStatusText(expression.status);
    document.getElementById('expr-result').textContent = expression.result !== undefined ? expression.result : '-';
    document.getElementById('expr-date').textContent = formatDate(expression.created_at || new Date().toISOString());
}

// Получение текста статуса
function getStatusText(status) {
    switch (status) {
        case 'completed':
            return 'Завершено';
        case 'pending':
            return 'В процессе';
        case 'error':
            return 'Ошибка';
        default:
            return status || 'Неизвестно';
    }
}

// Форматирование даты
function formatDate(dateString) {
    const date = new Date(dateString);
    return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}

// Загрузка задач выражения
async function loadExpressionTasks(exprId) {
    try {
        const response = await fetchWithAuth(`/api/expression/${exprId}/tasks`);
        
        if (!response.ok) {
            throw new Error('Не удалось загрузить задачи выражения');
        }
        
        const data = await response.json();
        renderTasks(data.tasks || []);
    } catch (error) {
        console.error('Ошибка при загрузке задач выражения:', error);
        document.getElementById('tasks-body').innerHTML = `
            <tr>
                <td colspan="7" class="error-message">
                    Не удалось загрузить детали вычисления: ${error.message}
                </td>
            </tr>
        `;
    }
}

// Отображение задач выражения
function renderTasks(tasks) {
    const tbody = document.getElementById('tasks-body');
    tbody.innerHTML = '';
    
    if (tasks.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="7" class="empty-table">Нет доступных задач для этого выражения</td>
            </tr>
        `;
        return;
    }
    
    // Сортируем задачи по ID, чтобы они отображались в правильном порядке
    tasks.sort((a, b) => a.id - b.id);
    
    tasks.forEach(task => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${task.id}</td>
            <td>${task.operation}</td>
            <td>${task.arg1}</td>
            <td>${task.arg2}</td>
            <td>${task.result !== undefined ? task.result : '-'}</td>
            <td>${getStatusText(task.status)}</td>
            <td>${task.execution_time ? `${task.execution_time} мс` : '-'}</td>
        `;
        tbody.appendChild(row);
    });
}

// Пересчет выражения
async function recalculateExpression() {
    try {
        const exprId = getExpressionId();
        const response = await fetchWithAuth(`/api/expression/${exprId}/recalculate`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            throw new Error('Не удалось запустить пересчет выражения');
        }
        
        alert('Выражение отправлено на пересчет');
        setTimeout(() => {
            loadExpressionDetails();
        }, 2000);
    } catch (error) {
        console.error('Ошибка при пересчете выражения:', error);
        alert('Ошибка при пересчете выражения: ' + error.message);
    }
}

// Отправка ссылки на выражение
function shareExpression() {
    const url = window.location.href;
    
    if (navigator.clipboard) {
        navigator.clipboard.writeText(url)
            .then(() => {
                alert('Ссылка скопирована в буфер обмена');
            })
            .catch(err => {
                console.error('Не удалось скопировать ссылку: ', err);
                promptManualCopy(url);
            });
    } else {
        promptManualCopy(url);
    }
}

// Запрос на ручное копирование ссылки
function promptManualCopy(url) {
    const textArea = document.createElement('textarea');
    textArea.value = url;
    document.body.appendChild(textArea);
    textArea.select();
    
    try {
        document.execCommand('copy');
        alert('Ссылка скопирована в буфер обмена');
    } catch (err) {
        console.error('Не удалось скопировать ссылку: ', err);
        alert('Скопируйте ссылку вручную: ' + url);
    }
    
    document.body.removeChild(textArea);
}

// Инициализация страницы
document.addEventListener('DOMContentLoaded', function() {
    // Проверяем авторизацию
    if (!currentToken) {
        window.location.href = '/auth';
        return;
    }
    
    // Загружаем детали выражения
    loadExpressionDetails();
    
    // Обработчики кнопок
    const recalculateBtn = document.getElementById('recalculate-btn');
    if (recalculateBtn) {
        recalculateBtn.addEventListener('click', recalculateExpression);
    }
    
    const shareBtn = document.getElementById('share-btn');
    if (shareBtn) {
        shareBtn.addEventListener('click', shareExpression);
    }
});
