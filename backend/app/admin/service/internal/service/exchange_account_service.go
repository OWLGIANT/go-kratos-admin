package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-admin/app/admin/service/internal/data"

	adminV1 "go-wind-admin/api/gen/go/admin/service/v1"
	tradingV1 "go-wind-admin/api/gen/go/trading/service/v1"
)

type ExchangeAccountService struct {
	adminV1.ExchangeAccountServiceHTTPServer

	log *log.Helper

	exchangeAccountRepo data.ExchangeAccountRepo
}

func NewExchangeAccountService(
	ctx *bootstrap.Context,
	exchangeAccountRepo data.ExchangeAccountRepo,
) *ExchangeAccountService {
	svc := &ExchangeAccountService{
		log:                 ctx.NewLoggerHelper("exchange-account/service/admin-service"),
		exchangeAccountRepo: exchangeAccountRepo,
	}

	return svc
}

// ListExchangeAccount 获取交易账号列表
func (s *ExchangeAccountService) ListExchangeAccount(ctx context.Context, req *paginationV1.PagingRequest) (*tradingV1.ListExchangeAccountResponse, error) {
	return s.exchangeAccountRepo.List(ctx, req)
}

// GetExchangeAccount 获取交易账号
func (s *ExchangeAccountService) GetExchangeAccount(ctx context.Context, req *tradingV1.GetExchangeAccountRequest) (*tradingV1.ExchangeAccount, error) {
	return s.exchangeAccountRepo.Get(ctx, req)
}

// CreateExchangeAccount 创建交易账号
func (s *ExchangeAccountService) CreateExchangeAccount(ctx context.Context, req *tradingV1.CreateExchangeAccountRequest) (*emptypb.Empty, error) {
	// 敏感信息加密在Repository层处理
	_, err := s.exchangeAccountRepo.Create(ctx, req)
	if err != nil {
		s.log.Errorf("create exchange account failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// UpdateExchangeAccount 更新交易账号
func (s *ExchangeAccountService) UpdateExchangeAccount(ctx context.Context, req *tradingV1.UpdateExchangeAccountRequest) (*emptypb.Empty, error) {
	// 敏感信息加密在Repository层处理
	err := s.exchangeAccountRepo.Update(ctx, req)
	if err != nil {
		s.log.Errorf("update exchange account failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// DeleteExchangeAccount 删除交易账号
func (s *ExchangeAccountService) DeleteExchangeAccount(ctx context.Context, req *tradingV1.DeleteExchangeAccountRequest) (*emptypb.Empty, error) {
	err := s.exchangeAccountRepo.Delete(ctx, req)
	if err != nil {
		s.log.Errorf("delete exchange account failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// BatchDeleteExchangeAccount 批量删除交易账号
func (s *ExchangeAccountService) BatchDeleteExchangeAccount(ctx context.Context, req *tradingV1.BatchDeleteExchangeAccountRequest) (*emptypb.Empty, error) {
	err := s.exchangeAccountRepo.BatchDelete(ctx, req)
	if err != nil {
		s.log.Errorf("batch delete exchange accounts failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// TransferExchangeAccount 转移交易账号
func (s *ExchangeAccountService) TransferExchangeAccount(ctx context.Context, req *tradingV1.TransferExchangeAccountRequest) (*emptypb.Empty, error) {
	err := s.exchangeAccountRepo.Transfer(ctx, req)
	if err != nil {
		s.log.Errorf("transfer exchange accounts failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// SearchExchangeAccount 搜索交易账号
func (s *ExchangeAccountService) SearchExchangeAccount(ctx context.Context, req *tradingV1.SearchExchangeAccountRequest) (*tradingV1.ListExchangeAccountResponse, error) {
	return s.exchangeAccountRepo.Search(ctx, req)
}

// GetAccountEquity 获取账号资金曲线
func (s *ExchangeAccountService) GetAccountEquity(ctx context.Context, req *tradingV1.GetAccountEquityRequest) (*tradingV1.GetAccountEquityResponse, error) {
	// TODO: 实现资金曲线查询逻辑
	// 这需要从其他数据源（如时序数据库）获取历史资金数据
	return &tradingV1.GetAccountEquityResponse{
		DataPoints: []*tradingV1.EquityDataPoint{},
	}, nil
}

// CreateCombinedAccount 创建组合账号
func (s *ExchangeAccountService) CreateCombinedAccount(ctx context.Context, req *tradingV1.CreateCombinedAccountRequest) (*emptypb.Empty, error) {
	_, err := s.exchangeAccountRepo.CreateCombined(ctx, req)
	if err != nil {
		s.log.Errorf("create combined account failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// UpdateCombinedAccount 更新组合账号
func (s *ExchangeAccountService) UpdateCombinedAccount(ctx context.Context, req *tradingV1.UpdateCombinedAccountRequest) (*emptypb.Empty, error) {
	err := s.exchangeAccountRepo.UpdateCombined(ctx, req)
	if err != nil {
		s.log.Errorf("update combined account failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// UpdateAccountRemark 更新账号备注
func (s *ExchangeAccountService) UpdateAccountRemark(ctx context.Context, req *tradingV1.UpdateAccountRemarkRequest) (*emptypb.Empty, error) {
	err := s.exchangeAccountRepo.UpdateRemark(ctx, req)
	if err != nil {
		s.log.Errorf("update account remark failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// UpdateAccountBrokerId 更新账号经纪商ID
func (s *ExchangeAccountService) UpdateAccountBrokerId(ctx context.Context, req *tradingV1.UpdateAccountBrokerIdRequest) (*emptypb.Empty, error) {
	err := s.exchangeAccountRepo.UpdateBrokerId(ctx, req)
	if err != nil {
		s.log.Errorf("update account broker id failed: %s", err.Error())
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
