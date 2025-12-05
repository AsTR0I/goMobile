package logic_test

// Разобраться с тестами

import "gomobile/internal/types"

type mockPolicyRepo struct{}

func (m *mockPolicyRepo) FindBestPolicy(numA, numB, numC string, unixTime int64, srcIP, sbcIP, callID string) *types.Policy {
	return &types.Policy{}
}
