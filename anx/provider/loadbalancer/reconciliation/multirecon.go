package reconciliation

import (
	"sync"

	"go.anx.io/go-anxcloud/pkg/api/types"
)

// MultiReconciliation is a collection of Reconcilation to do concurrently.
type MultiReconciliation interface {
	// Add another Reconcilation to the collection of reconciliations to do.
	Add(Reconciliation)

	Reconciliation
}

type multirecon struct {
	recons []Reconciliation
}

// Multi creates a new MultiReconcilation from the given Reconcilation instances. Instead of
// giving them as attributes, they can also be added later via Add().
//
// The ReconcileCheck, Reconcile and Status methods are called on all reconciliations in the collection
// concurrently and the results are aggregated into the returned values.
//
// For Reconcile and ReconcileCheck this means, reconciliation is complete after all wrapped reconciliations
// are complete.
//
// Status returns the set of addresses and ports given by all wrapped reconciliations, those only one some
// reconciliations are removed from the returned value.
//
// This can be used to e.g. reconcile a given service for multiple different LBaaS LoadBalancers, removing
// the need for the LBaaS sync controller.
func Multi(recon ...Reconciliation) MultiReconciliation {
	return &multirecon{
		recons: recon,
	}
}

func (mr *multirecon) Add(recon Reconciliation) {
	if mr.recons == nil {
		mr.recons = make([]Reconciliation, 0, 1)
	}

	mr.recons = append(mr.recons, recon)
}

func (mr *multirecon) ReconcileCheck() ([]types.Object, []types.Object, error) {
	type singleResult struct {
		toCreate  []types.Object
		toDestroy []types.Object
		err       error
	}

	toCreate := make([]types.Object, 0)
	toDestroy := make([]types.Object, 0)

	wg := sync.WaitGroup{}
	wg.Add(len(mr.recons))

	results := make(chan singleResult, len(mr.recons))
	for i := range mr.recons {
		recon := mr.recons[i]
		go func() {
			defer wg.Done()
			toCreate, toDestroy, err := recon.ReconcileCheck()

			results <- singleResult{
				toCreate:  toCreate,
				toDestroy: toDestroy,
				err:       err,
			}
		}()
	}

	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			return nil, nil, result.err
		}

		toCreate = append(toCreate, result.toCreate...)
		toDestroy = append(toDestroy, result.toDestroy...)
	}

	return toCreate, toDestroy, nil
}

func (mr *multirecon) Reconcile() error {
	wg := sync.WaitGroup{}
	wg.Add(len(mr.recons))

	results := make(chan error, len(mr.recons))
	for i := range mr.recons {
		recon := mr.recons[i]
		go func() {
			defer wg.Done()
			results <- recon.Reconcile()
		}()
	}

	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			return err
		}
	}

	return nil
}

func (mr *multirecon) Status() (map[string][]uint16, error) {
	type singleResult struct {
		status map[string][]uint16
		err    error
	}

	wg := sync.WaitGroup{}
	wg.Add(len(mr.recons))

	results := make(chan singleResult, len(mr.recons))
	for i := range mr.recons {
		recon := mr.recons[i]
		go func() {
			defer wg.Done()
			status, err := recon.Status()

			results <- singleResult{
				status: status,
				err:    err,
			}
		}()
	}

	wg.Wait()
	close(results)

	status := make([]map[string][]uint16, 0, len(mr.recons))

	for result := range results {
		if result.err != nil {
			return nil, result.err
		}

		status = append(status, result.status)
	}

	return mergeReconStatus(status), nil

}

func mergeReconStatus(status []map[string][]uint16) map[string][]uint16 {
	addressPortReturnedCount := make(map[string]map[uint16]int)

	// first track number of status having a address-port-combination
	for _, s := range status {
		for address, ports := range s {
			if _, ok := addressPortReturnedCount[address]; !ok {
				addressPortReturnedCount[address] = make(map[uint16]int)
			}

			for _, port := range ports {
				addressPortReturnedCount[address][port]++
			}
		}
	}

	ret := make(map[string][]uint16)

	for addr, portCount := range addressPortReturnedCount {
		maxPortCount := 0
		for _, count := range portCount {
			if count > maxPortCount {
				maxPortCount = count
			}
		}

		// at least one port in address is returned in every status
		if maxPortCount == len(status) {
			ports := make([]uint16, 0)
			for port, count := range portCount {
				if count == len(status) {
					ports = append(ports, port)
				}
			}

			ret[addr] = ports
		}
	}

	return ret
}
