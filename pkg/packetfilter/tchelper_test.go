package packetfilter

import (
	"firedocker/pkg/packetfilter/mocks"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:generate mockery --name=execTcHelper --structname=ExecTcHelper

func TestEnsureBailsExistingQueue(t *testing.T) {
	helper := new(mocks.ExecTcHelper)
	helper.On("Execute", "qdisc", "show", "dev", "fake0").Return("qdisc fq_codel 0: dev fake0 root refcnt 2 limit 10240p flows 1024 quantum 1500 target 5.0ms interval 100.0ms memory_limit 32Mb ecn\n", nil)

	tcHelper := &tcHelperImpl{}

	res := tcHelper.ensureQdiscClsact("fake0", helper.Execute)

	require.NotNil(t, res)
	helper.AssertExpectations(t)
}

func TestEnsureDoesNotReapply(t *testing.T) {
	helper := new(mocks.ExecTcHelper)
	helper.On("Execute", "qdisc", "show", "dev", "fake0").Return("qdisc clsact ffff: dev fake0 parent ffff:fff1\n", nil)

	tcHelper := &tcHelperImpl{}

	res := tcHelper.ensureQdiscClsact("fake0", helper.Execute)

	require.Nil(t, res)
	helper.AssertExpectations(t)
}

func TestEnsureApplies(t *testing.T) {
	helper := new(mocks.ExecTcHelper)
	helper.On("Execute", "qdisc", "show", "dev", "fake0").Return("qdisc noqueue 0: dev fake0 root refcnt 2\n", nil)
	helper.On("Execute", "qdisc", "add", "dev", "fake0", "clsact").Return("\n", nil)
	tcHelper := &tcHelperImpl{}

	res := tcHelper.ensureQdiscClsact("fake0", helper.Execute)

	require.Nil(t, res)
	helper.AssertExpectations(t)
}

func TestInsertsFilter(t *testing.T) {
	helper := new(mocks.ExecTcHelper)
	helper.On("Execute", "filter", "del", "dev", "fake0", "ingress").Return("\n", nil)
	helper.On("Execute", "filter", "add", "dev", "fake0", "ingress", "bpf", "da", "obj", "/fake/path.bpf", "sec", "ingress").Return("\n", nil)
	tcHelper := &tcHelperImpl{}

	res := tcHelper.loadBPFIngress("fake0", "/fake/path.bpf", helper.Execute)

	require.Nil(t, res)
	helper.AssertExpectations(t)
}
