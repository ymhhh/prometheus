// GNU GPL v3 License
// Copyright (c) 2019 github.com:go-trellis

package txorm

import (
	"github.com/go-trellis/common/errors"
)

// TX do transaction function by default database
func (p *TXorm) TX(fn interface{}, repos ...interface{}) errors.ErrorCode {
	return p.TXWithName(fn, DefaultDatabase, repos...)
}

// TXWithName do transaction function with name of database
func (p *TXorm) TXWithName(fn interface{}, name string, repos ...interface{}) errors.ErrorCode {
	if err := p.checkRepos(fn, repos); err != nil {
		return err
	}

	_newRepos := []interface{}{}
	_newTXormRepos := []*TXorm{}

	for _, origin := range repos {

		repo := getRepo(origin)
		if repo == nil {
			return ErrStructCombineWithRepo.New(errors.Params{"name": "tgrom"})
		}

		_newTxorm, newRepoI, err := createNewTXorm(origin)
		if err != nil {
			return err
		}

		_newTxorm.engines = repo.engines
		_newTxorm.defEngine = repo.defEngine
		_newRepos = append(_newRepos, newRepoI)
		_newTXormRepos = append(_newTXormRepos, _newTxorm)
	}

	if err := _newTXormRepos[0].beginTransaction(name); err != nil {
		return err
	}

	for i := range _newTXormRepos {
		_newTXormRepos[i].txSession = _newTXormRepos[0].txSession
		_newTXormRepos[i].isTransaction = _newTXormRepos[0].isTransaction
	}

	return _newTXormRepos[0].commitTransaction(fn, _newRepos...)
}

func (p *TXorm) beginTransaction(name string) errors.ErrorCode {
	if !p.isTransaction {
		p.isTransaction = true
		_engine, err := p.getEngine(name)
		if err != nil {
			return err
		}
		p.txSession = _engine.NewSession()
		if p.txSession == nil {
			return ErrTransactionSessionIsNil.New()
		}
		return nil
	}
	return ErrTransactionIsAlreadyBegin.New(errors.Params{"name": name})
}

func (p *TXorm) commitTransaction(txFunc interface{}, repos ...interface{}) errors.ErrorCode {
	if !p.isTransaction {
		return ErrNonTransactionCantCommit.New()
	}

	if p.txSession == nil {
		return ErrTransactionSessionIsNil.New()
	}
	defer p.txSession.Close()

	if txFunc == nil {
		return ErrNotFoundTransationFunction.New()
	}

	isNeedRollBack := true

	if err := p.txSession.Begin(); err != nil {
		return ErrFailToCreateTransaction.New().Append(err)
	}

	defer func() {
		if isNeedRollBack {
			_ = p.txSession.Rollback()
		}
	}()

	_funcs := GetLogicFuncs(txFunc)

	var (
		_values []interface{}
		ecode   errors.ErrorCode
	)

	if _funcs.BeforeLogic != nil {
		if _, ecode = CallFunc(_funcs.BeforeLogic, _funcs, repos); ecode != nil {
			return ecode
		}
	}

	if _funcs.Logic != nil {
		if _values, ecode = CallFunc(_funcs.Logic, _funcs, repos); ecode != nil {
			return ecode
		}
	}

	if _funcs.AfterLogic != nil {
		if _values, ecode = CallFunc(_funcs.AfterLogic, _funcs, repos); ecode != nil {
			return ecode
		}
	}

	isNeedRollBack = false
	if err := p.txSession.Commit(); err != nil {
		return ErrFailToCommitTransaction.New().Append(err)
	}

	if _funcs.AfterCommit != nil {
		if _, ecode = CallFunc(_funcs.AfterCommit, _funcs, _values); ecode != nil {
			return ecode
		}
	}

	return nil
}
