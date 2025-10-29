document.addEventListener('DOMContentLoaded', () => {
    // --- Получаем ссылки на ключевые элементы DOM ---
    const searchButton = document.getElementById('searchButton');
    const orderUidInput = document.getElementById('orderUidInput');
    const resultContainer = document.getElementById('resultContainer');
    const errorContainer = document.getElementById('errorContainer');
    const loader = document.getElementById('loader');

    // --- Функции для управления UI ---
    const showLoader = () => loader.classList.remove('is-hidden');
    const hideLoader = () => loader.classList.add('is-hidden');

    const showError = (message) => {
        errorContainer.textContent = message;
        errorContainer.classList.remove('is-hidden');
    };

    const hideError = () => errorContainer.classList.add('is-hidden');

    const clearResults = () => {
        resultContainer.innerHTML = '';
        resultContainer.classList.add('is-hidden');
    };

    // --- Основная функция для поиска заказа ---
    async function fetchOrderData() {
        const orderUid = orderUidInput.value.trim();
        if (!orderUid) return;

        // Подготовка UI к запросу
        showLoader();
        searchButton.disabled = true;
        hideError();
        clearResults();

        try {
            const response = await fetch(`/api/order/${orderUid}`);
            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(`Ошибка: ${errorText} (Статус: ${response.status})`);
            }
            const orderData = await response.json();
            renderOrderData(orderData);
        } catch (error) {
            showError(error.message);
        } finally {
            // Возвращаем UI в исходное состояние
            hideLoader();
            searchButton.disabled = orderUidInput.value.trim() === '';
        }
    }

    // --- Функция для отрисовки полученных данных на странице ---
    function renderOrderData(order) {
        // Хелпер для создания элемента с меткой и значением
        const createGridItem = (label, value) => {
            const itemElement = document.createElement('div');
            itemElement.className = 'grid-item';

            const labelSpan = document.createElement('span');
            labelSpan.className = 'grid-item-label';
            labelSpan.textContent = `${label}: `;

            itemElement.appendChild(labelSpan);
            itemElement.append(value || 'не указано');
            return itemElement;
        };

        // Хелпер для создания информационной карточки
        const createInfoCard = (title, ...childElements) => {
            const card = document.createElement('div');
            card.className = 'info-card';
            const cardTitle = document.createElement('h2');
            cardTitle.className = 'info-card-title';
            cardTitle.textContent = title;
            const grid = document.createElement('div');
            grid.className = 'info-grid';

            childElements.forEach(element => grid.appendChild(element));
            card.append(cardTitle, grid);
            return card;
        };

        // Создание и добавление карточек с информацией о заказе
        const generalInfoCard = createInfoCard('Общая информация',
            createGridItem('UID Заказа', order.order_uid),
            createGridItem('Трек-номер', order.track_number),
            createGridItem('ID Покупателя', order.customer_id),
            createGridItem('Дата создания', new Date(order.date_created).toLocaleString())
        );

        const deliveryCard = createInfoCard('Адрес доставки',
            createGridItem('Имя получателя', order.delivery.name),
            createGridItem('Телефон', order.delivery.phone),
            createGridItem('Город', order.delivery.city),
            createGridItem('Адрес', order.delivery.address)
        );

        const paymentCard = createInfoCard('Детали оплаты',
            createGridItem('ID Транзакции', order.payment.transaction),
            createGridItem('Сумма', `${order.payment.amount} ${order.payment.currency}`),
            createGridItem('Банк', order.payment.bank)
        );

        // Отдельно создаем карточку для товаров
        const itemsCard = document.createElement('div');
        itemsCard.className = 'info-card';
        const itemsTitle = document.createElement('h2');
        itemsTitle.className = 'info-card-title';
        itemsTitle.textContent = 'Состав заказа';
        itemsCard.appendChild(itemsTitle);

        order.items.forEach(item => {
            const itemContainer = document.createElement('div');
            itemContainer.className = 'item-card';
            const grid = document.createElement('div');
            grid.className = 'info-grid';
            grid.append(
                createGridItem('Название', item.name),
                createGridItem('Бренд', item.brand),
                createGridItem('Цена', item.total_price)
            );
            itemContainer.appendChild(grid);
            itemsCard.appendChild(itemContainer);
        });

        resultContainer.append(generalInfoCard, deliveryCard, paymentCard, itemsCard);
        resultContainer.classList.remove('is-hidden');
    }

    // --- Назначаем обработчики событий ---
    searchButton.addEventListener('click', fetchOrderData);
    orderUidInput.addEventListener('keypress', (event) => {
        if (event.key === 'Enter') {
            fetchOrderData();
        }
    });
    orderUidInput.addEventListener('input', () => {
        searchButton.disabled = orderUidInput.value.trim() === '';
    });
});