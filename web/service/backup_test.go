package service

import (
	"os"
	"testing"
)

func TestBackupDB(t *testing.T) {
	testDBPath := "/home/trd12/GolandProjects/3x-ui/db/x-ui.db"
	backupDir := "/home/trd12/GolandProjects/3x-ui/backupplace"

	file, err := os.Create(testDBPath)
	if err != nil {
		t.Fatalf("Ошибка создания тестовой БД: %v", err)
	}
	file.Close()
	//defer os.Remove(testDBPath)

	err = BackupDB(testDBPath, backupDir)
	if err != nil {
		t.Errorf("Ошибка выполнения BackupDB: %v", err)
	}
}
