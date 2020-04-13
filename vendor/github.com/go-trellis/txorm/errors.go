// GNU GPL v3 License
// Copyright (c) 2019 github.com:go-trellis

package txorm

import "github.com/go-trellis/common/errors"

const _namespace = "go-trellis::txorm"

// define connector errors
var (
	ErrFailToExecuteLogicFunction = errors.TN(_namespace, 10001, "{{.message}}")

	// define orm errors
	ErrNotFoundDefaultDatabase    = errors.TN(_namespace, 20000, "not found default database")
	ErrAtLeastOneRepo             = errors.TN(_namespace, 20001, "input one repo at least")
	ErrNotFoundTransationFunction = errors.TN(_namespace, 20002, "not found transation function")
	ErrStructCombineWithRepo      = errors.TN(_namespace, 20003, "your repository struct should combine with {{.name}} repo")
	ErrFailToCreateRepo           = errors.TN(_namespace, 20004, "fail to create an new repo")
	ErrFailToDerive               = errors.TN(_namespace, 20005, "fail to derive: {{.message}}")
	ErrFailToInherit              = errors.TN(_namespace, 20006, "fail to inherit: {{.message}}")
	ErrFailToConvetTXToNonTX      = errors.TN(_namespace, 20007, "could not convert TX to NON-TX")
	ErrFailToCreateTransaction    = errors.TN(_namespace, 20008, "fail to create transaction: {{.message}}")
	ErrTransactionIsAlreadyBegin  = errors.TN(_namespace, 20009, "transaction is already begin: {{.name}}")
	ErrNonTransactionCantCommit   = errors.TN(_namespace, 20010, "non-transaction can't commit")
	ErrTransactionSessionIsNil    = errors.TN(_namespace, 20011, "transaction session is nil")
	ErrFailToCommitTransaction    = errors.TN(_namespace, 20012, "fail to commit transaction")
	ErrNotFoundXormEngine         = errors.TN(_namespace, 20013, "not found xorm engine with {{.name}}")
)
