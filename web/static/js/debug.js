// Отладочный файл для диагностики проблем с API и интерфейсом

function debugApiRequest(method, url, body = null, headers = {}) {
    console.log(`--- Sending ${method} request to ${url} ---`);
    if (body) console.log('Body:', body);
    if (headers) console.log('Headers:', headers);
    
    const options = {
        method: method,
        headers: headers,
    };
    
    if (body) {
        options.body = JSON.stringify(body);
    }
    
    return fetch(url, options)
        .then(response => {
            console.log(`Response status: ${response.status} ${response.statusText}`);
            return response.json().then(data => {
                console.log('Response data:', data);
                return { status: response.status, ok: response.ok, data: data };
            }).catch(err => {
                console.error('Error parsing JSON:', err);
                return { status: response.status, ok: response.ok, error: 'JSON parsing error' };
            });
        })
        .catch(error => {
            console.error('Network error:', error);
            return { error: error.message };
        });
}

function debugLocalStorage() {
    console.log('--- Local Storage Debug ---');
    console.log('Token:', localStorage.getItem('token'));
    console.log('Calculated Expressions:', localStorage.getItem('calculatedExpressions'));
    
    // Отобразить все элементы localStorage
    console.log('All localStorage items:');
    for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        console.log(`${key}: ${localStorage.getItem(key)}`);
    }
}

// Функция для тестирования всего API
async function testAllApi() {
    console.log('=== Starting API Tests ===');
    
    // Проверяем, есть ли токен
    const token = localStorage.getItem('token');
    if (!token) {
        console.log('No token found. Will try to login first.');
        await testLogin();
    } else {
        console.log('Token found:', token);
    }
    
    // Тестируем историю вычислений
    await testHistory();
    
    // Тестируем вычисление
    await testCalculation();
    
    console.log('=== API Tests Completed ===');
}

async function testLogin() {
    console.log('--- Testing Login ---');
    const result = await debugApiRequest('POST', '/api/v1/login', {
        login: 'testuser',
        password: 'testpassword'
    }, {
        'Content-Type': 'application/json'
    });
    
    if (result.ok && result.data.token) {
        localStorage.setItem('token', result.data.token);
        console.log('Login successful, token saved');
    } else {
        console.error('Login failed');
    }
}

async function testHistory() {
    console.log('--- Testing Expressions History ---');
    const token = localStorage.getItem('token');
    if (!token) {
        console.error('No token available for history test');
        return;
    }
    
    const result = await debugApiRequest('GET', '/api/v1/expressions', null, {
        'Authorization': `Bearer ${token}`
    });
    
    if (result.ok) {
        console.log('History retrieved successfully');
    } else {
        console.error('History retrieval failed');
    }
}

async function testCalculation() {
    console.log('--- Testing Calculation ---');
    const token = localStorage.getItem('token');
    if (!token) {
        console.error('No token available for calculation test');
        return;
    }
    
    const result = await debugApiRequest('POST', '/api/v1/calculate', {
        expression: '3+3'
    }, {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
    });
    
    if (result.ok && result.data.id) {
        console.log('Calculation submitted successfully, expression ID:', result.data.id);
        
        // Сохраняем выражение в localStorage
        const calculatedExpressions = JSON.parse(localStorage.getItem('calculatedExpressions') || '{}');
        calculatedExpressions[result.data.id] = '3+3';
        localStorage.setItem('calculatedExpressions', JSON.stringify(calculatedExpressions));
        
        // Проверяем статус выражения
        await checkExpressionStatus(result.data.id);
    } else {
        console.error('Calculation failed');
    }
}

async function checkExpressionStatus(id) {
    console.log(`--- Checking Expression Status for ${id} ---`);
    const token = localStorage.getItem('token');
    if (!token) {
        console.error('No token available for status check');
        return;
    }
    
    const result = await debugApiRequest('GET', `/api/v1/expressions/${id}`, null, {
        'Authorization': `Bearer ${token}`
    });
    
    if (result.ok) {
        console.log('Status retrieved successfully');
    } else {
        console.error('Status retrieval failed');
    }
}

// Функция для сброса и очистки всех данных
function resetAll() {
    console.log('--- Resetting All Data ---');
    localStorage.clear();
    console.log('Local storage cleared');
} 