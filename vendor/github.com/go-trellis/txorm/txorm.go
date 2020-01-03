// GNU GPL v3 License
// Copyright (c) 2019 github.com:go-trellis

package txorm

import (
	"reflect"

	"github.com/go-trellis/errors"
	"github.com/go-xorm/xorm"
)

// TXorm trellis xorm
type TXorm struct {
	isTransaction bool
	txSession     *xorm.Session

	engines   map[string]*xorm.Engine
	defEngine *xorm.Engine
}

// New get trellis xorm committer
func New() Committer {
	return &TXorm{}
}

// SetEngines set xorm engines
func (p *TXorm) SetEngines(engines map[string]*xorm.Engine) {
	if defEngine, exist := engines[DefaultDatabase]; exist {
		p.engines = engines
		p.defEngine = defEngine
	} else {
		panic(ErrNotFoundDefaultDatabase.New())
	}
}

// Session get session
func (p *TXorm) Session() *xorm.Session {
	return p.txSession
}

// GetEngine get engine by name
func (p *TXorm) getEngine(name string) (*xorm.Engine, errors.ErrorCode) {
	if engine, _exist := p.engines[name]; _exist {
		return engine, nil
	}
	return nil, ErrNotFoundXormEngine.New(errors.Params{"name": name})
}

func (p *TXorm) checkRepos(txFunc interface{}, originRepos ...interface{}) errors.ErrorCode {
	if reposLen := len(originRepos); reposLen < 1 {
		return ErrAtLeastOneRepo.New()
	}

	if txFunc == nil {
		return ErrNotFoundTransationFunction.New()
	}
	return nil
}

func getRepo(v interface{}) *TXorm {
	_deepRepo := DeepFields(v, reflect.TypeOf(new(TXorm)), []reflect.Value{})
	if deepRepo, ok := _deepRepo.(*TXorm); ok {
		return deepRepo
	}
	return nil
}

func createNewTXorm(origin interface{}) (*TXorm, interface{}, errors.ErrorCode) {
	if repo, err := Derive(origin); err != nil {
		return nil, nil, ErrFailToDerive.New(errors.Params{"message": err.Error()})
	} else if repo != nil {
		return getRepo(repo), repo, nil
	}

	newRepoV := reflect.New(reflect.ValueOf(
		reflect.Indirect(reflect.ValueOf(origin)).Interface()).Type())
	if !newRepoV.IsValid() {
		return nil, nil, ErrFailToCreateRepo.New()
	}

	newRepoI := newRepoV.Interface()

	if err := Inherit(newRepoI, origin); err != nil {
		return nil, nil, ErrFailToInherit.New(errors.Params{"message": err.Error()})
	}

	newTxorm := getRepo(newRepoI)

	if newTxorm == nil {
		return nil, nil, ErrFailToConvetTXToNonTX.New()
	}
	return newTxorm, newRepoI, nil
}
