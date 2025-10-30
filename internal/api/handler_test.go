package api

import (
	"L0_project/internal/cache/mocks"
	db_mocks "L0_project/internal/database/mocks"
	"L0_project/internal/model"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// helperTestOrder - универсальный тестовый заказ
var helperTestOrder = &model.Order{
	OrderUID:    "test-uid-123",
	TrackNumber: "track-123",
	Delivery: model.Delivery{
		Name: "Test User",
	},
	Items: []model.Item{
		{Name: "Test Item"},
	},
}

// setupHandlerAndMocks - хелпер для инициализации хендлера и моков
func setupHandlerAndMocks(t *testing.T) (*gomock.Controller, *OrderHandler, *mocks.MockCache, *db_mocks.MockStorage) {
	ctrl := gomock.NewController(t)
	mockCache := mocks.NewMockCache(ctrl)
	mockStorage := db_mocks.NewMockStorage(ctrl)
	handler := NewOrderHandler(mockStorage, mockCache)
	return ctrl, handler, mockCache, mockStorage
}

// createTestRequest - хелпер для создания HTTP-запроса с URL-параметром
func createTestRequest(t *testing.T, uid string) *http.Request {
	req := httptest.NewRequest("GET", "/api/order/"+uid, nil)

	// Контекст chi для URL-параметров
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orderUID", uid)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	return req
}

func TestOrderHandler_GetByUID_CacheHit(t *testing.T) {
	ctrl, handler, mockCache, mockStorage := setupHandlerAndMocks(t)
	defer ctrl.Finish()

	uid := "test-uid-123"
	rr := httptest.NewRecorder()
	req := createTestRequest(t, uid)

	// Ожидаем вызов кэша
	mockCache.EXPECT().Get(uid).Return(helperTestOrder, true)
	// Не ожидаем вызова БД
	mockStorage.EXPECT().GetOrderByUID(gomock.Any(), gomock.Any()).Times(0)

	handler.GetByUID(rr, req)

	// Проверка ответа
	assert.Equal(t, http.StatusOK, rr.Code)

	var order model.Order
	err := json.Unmarshal(rr.Body.Bytes(), &order)
	assert.NoError(t, err)
	assert.Equal(t, helperTestOrder.OrderUID, order.OrderUID)
}

func TestOrderHandler_GetByUID_CacheMiss_DBHit(t *testing.T) {
	ctrl, handler, mockCache, mockStorage := setupHandlerAndMocks(t)
	defer ctrl.Finish()

	uid := "test-uid-123"
	rr := httptest.NewRecorder()
	req := createTestRequest(t, uid)

	// 1. Ожидаем промах кэша
	mockCache.EXPECT().Get(uid).Return(nil, false)
	// 2. Ожидаем запрос к БД
	mockStorage.EXPECT().GetOrderByUID(gomock.Any(), uid).Return(helperTestOrder, nil)
	// 3. Ожидаем сохранение в кэш
	mockCache.EXPECT().Set(uid, helperTestOrder).Times(1)

	handler.GetByUID(rr, req)

	// Проверка ответа
	assert.Equal(t, http.StatusOK, rr.Code)

	var order model.Order
	err := json.Unmarshal(rr.Body.Bytes(), &order)
	assert.NoError(t, err)
	assert.Equal(t, helperTestOrder.OrderUID, order.OrderUID)
}

func TestOrderHandler_GetByUID_NotFound(t *testing.T) {
	ctrl, handler, mockCache, mockStorage := setupHandlerAndMocks(t)
	defer ctrl.Finish()

	uid := "not-found-uid"
	rr := httptest.NewRecorder()
	req := createTestRequest(t, uid)

	// 1. Ожидаем промах кэша
	mockCache.EXPECT().Get(uid).Return(nil, false)
	// 2. Ожидаем запрос к БД, который вернет ошибку
	mockStorage.EXPECT().GetOrderByUID(gomock.Any(), uid).Return(nil, errors.New("not found"))
	// 3. Не ожидаем вызова Set в кэш
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any()).Times(0)

	handler.GetByUID(rr, req)

	// Проверка ответа
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestOrderHandler_GetByUID_NoUID(t *testing.T) {
	_, handler, _, _ := setupHandlerAndMocks(t)

	// Создаем запрос без chi-контекста
	req := httptest.NewRequest("GET", "/api/order/", nil)
	rr := httptest.NewRecorder()

	handler.GetByUID(rr, req)

	// Проверка ответа
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
