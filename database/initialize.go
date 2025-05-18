package database

import (
	"zatrano/configs/logconfig"
	"zatrano/database/migrations"
	"zatrano/database/seeders"
	"zatrano/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func Initialize(db *gorm.DB, migrate bool, seed bool) {
	if !migrate && !seed {
		logconfig.SLog.Info("Migrate veya seed bayrağı belirtilmedi, işlem yapılmayacak.")
		return
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			logconfig.Log.Fatal("Veritabanı başlatma işlemi başarısız oldu, geri alındı (panic)", zap.Any("panic_info", r))
		}
		if tx.Error != nil && tx.Error != gorm.ErrInvalidTransaction {
			logconfig.SLog.Warn("Başlatma sırasında hata oluştuğu için işlem geri alınıyor.")
			tx.Rollback()
		}
	}()

	logconfig.SLog.Info("Veritabanı başlatma işlemi başlıyor...")

	if migrate {
		logconfig.SLog.Info("Migrasyonlar çalıştırılıyor...")
		if err := RunMigrationsInOrder(tx); err != nil {
			tx.Rollback()
			logconfig.Log.Fatal("Migrasyon başarısız oldu", zap.Error(err))
		}
		logconfig.SLog.Info("Migrasyonlar tamamlandı.")
	} else {
		logconfig.SLog.Info("Migrate bayrağı belirtilmedi, migrasyon adımı atlanıyor.")
	}

	if seed {
		logconfig.SLog.Info("Seeder'lar çalıştırılıyor...")
		if err := CheckAndRunSeeders(tx); err != nil {
			tx.Rollback()
			logconfig.Log.Fatal("Seeding başarısız oldu", zap.Error(err))
		}
		logconfig.SLog.Info("Seeder'lar tamamlandı.")
	} else {
		logconfig.SLog.Info("Seed bayrağı belirtilmedi, seeder adımı atlanıyor.")
	}

	logconfig.SLog.Info("İşlem commit ediliyor...")
	if err := tx.Commit().Error; err != nil {
		logconfig.Log.Fatal("Commit başarısız oldu", zap.Error(err))
	}

	logconfig.SLog.Info("Veritabanı başlatma işlemi başarıyla tamamlandı")
}

func RunMigrationsInOrder(db *gorm.DB) error {
	if err := migrations.MigrateUsersTable(db); err != nil {
		return err
	}
	return nil
}

func CheckAndRunSeeders(db *gorm.DB) error {
	systemUser := seeders.GetSystemUserConfig()
	var existingUser models.User
	result := db.Where("email = ? AND type = ?", systemUser.Email, models.Dashboard).First(&existingUser)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			logconfig.SLog.Info("Sistem kullanıcısı oluşturuluyor: %s (%s)...", systemUser.Name, systemUser.Email)
			if err := seeders.SeedSystemUser(db); err != nil {
				logconfig.Log.Error("Sistem kullanıcısı seed edilemedi", zap.Error(err))
				return err
			}
			logconfig.SLog.Info(" -> Sistem kullanıcısı oluşturuldu.")
		} else {
			logconfig.Log.Error("Sistem kullanıcısı kontrol edilirken hata", zap.Error(result.Error))
			return result.Error
		}
	} else {
		logconfig.SLog.Info("Sistem kullanıcısı '%s' (%s) zaten mevcut, oluşturma adımı atlanıyor.",
			existingUser.Name, existingUser.Email)
		logconfig.SLog.Info("Mevcut sistem kullanıcısı '%s' için güncelleme kontrolü yapılıyor...", existingUser.Email)
		if err := seeders.SeedSystemUser(db); err != nil {
			logconfig.Log.Error("Mevcut sistem kullanıcısı güncellenirken/kontrol edilirken hata", zap.Error(err))
			return err
		}

	}
	return nil
}
