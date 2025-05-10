document.addEventListener('DOMContentLoaded', function() {
    // Переключение между вкладками справки
    const tabs = document.querySelectorAll('.help-tab');
    const tabContents = document.querySelectorAll('.help-tab-content');
    
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
            document.getElementById(tabId).classList.add('active');
        });
    });
});
