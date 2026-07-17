package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	sqlitePath := flag.String("sqlite", "one-api.db?_busy_timeout=30000", "SQLite source path")
	mysqlDSN := flag.String("mysql", "", "MySQL target DSN")
	batchSize := flag.Int("batch-size", 200, "rows per insert batch")
	wipeTarget := flag.Bool("wipe-target", true, "truncate target tables before import")
	flag.Parse()

	if strings.TrimSpace(*mysqlDSN) == "" {
		log.Fatal("missing required -mysql DSN")
	}
	if *batchSize <= 0 {
		log.Fatal("batch-size must be greater than 0")
	}

	sourceDB, err := gorm.Open(sqlite.Open(*sqlitePath), &gorm.Config{})
	if err != nil {
		log.Fatalf("open sqlite failed: %v", err)
	}

	targetDB, err := gorm.Open(mysql.Open(*mysqlDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("open mysql failed: %v", err)
	}

	if err := ensureSchema(targetDB); err != nil {
		log.Fatalf("ensure mysql schema failed: %v", err)
	}

	tables, err := loadSQLiteTables(sourceDB)
	if err != nil {
		log.Fatalf("load sqlite tables failed: %v", err)
	}

	if err := targetDB.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
		log.Fatalf("disable foreign key checks failed: %v", err)
	}
	defer func() {
		if err := targetDB.Exec("SET FOREIGN_KEY_CHECKS = 1").Error; err != nil {
			log.Printf("restore foreign key checks failed: %v", err)
		}
	}()

	startedAt := time.Now()
	totalRows := 0
	for _, table := range tables {
		rows, err := copyTable(sourceDB, targetDB, table, *batchSize, *wipeTarget)
		if err != nil {
			log.Fatalf("copy table %s failed: %v", table, err)
		}
		totalRows += rows
		log.Printf("migrated table %s: %d rows", table, rows)
	}

	log.Printf("sqlite to mysql migration completed: %d tables, %d rows, elapsed=%s", len(tables), totalRows, time.Since(startedAt).Round(time.Millisecond))
}

func ensureSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.Channel{},
		&model.Token{},
		&model.User{},
		&model.PasskeyCredential{},
		&model.Option{},
		&model.Redemption{},
		&model.Ability{},
		&model.Log{},
		&model.Midjourney{},
		&model.TopUp{},
		&model.QuotaData{},
		&model.Task{},
		&model.Model{},
		&model.Vendor{},
		&model.PrefillGroup{},
		&model.Setup{},
		&model.TwoFA{},
		&model.TwoFABackupCode{},
		&model.Checkin{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{},
		&model.CustomOAuthProvider{},
		&model.UserOAuthBinding{},
		&model.PerfMetric{},
		&model.FactoryAccountBinding{},
		&model.SubscriptionPlan{},
	); err != nil {
		return err
	}
	return nil
}

func loadSQLiteTables(db *gorm.DB) ([]string, error) {
	rows, err := db.Raw(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		  AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func copyTable(sourceDB *gorm.DB, targetDB *gorm.DB, table string, batchSize int, wipeTarget bool) (int, error) {
	if !targetDB.Migrator().HasTable(table) {
		return 0, fmt.Errorf("target table %s does not exist", table)
	}

	returnCount := 0
	err := targetDB.Transaction(func(tx *gorm.DB) error {
		if wipeTarget {
			if err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s", quoteIdent(table))).Error; err != nil {
				return fmt.Errorf("truncate target table failed: %w", err)
			}
		}

		sourceRows, err := sourceDB.Raw(fmt.Sprintf("SELECT * FROM %s", quoteIdent(table))).Rows()
		if err != nil {
			return fmt.Errorf("query source rows failed: %w", err)
		}
		defer sourceRows.Close()

		columns, err := sourceRows.Columns()
		if err != nil {
			return fmt.Errorf("load source columns failed: %w", err)
		}
		if len(columns) == 0 {
			return nil
		}

		rowPlaceholder := "(" + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ")"
		insertPrefix := fmt.Sprintf(
			"INSERT INTO %s(%s) VALUES ",
			quoteIdent(table),
			joinQuoted(columns),
		)

		valueClauses := make([]string, 0, batchSize)
		batchArgs := make([]interface{}, 0, batchSize*len(columns))

		flush := func() error {
			if len(valueClauses) == 0 {
				return nil
			}
			sqlText := insertPrefix + strings.Join(valueClauses, ",")
			if err := tx.Exec(sqlText, batchArgs...).Error; err != nil {
				return err
			}
			valueClauses = valueClauses[:0]
			batchArgs = batchArgs[:0]
			return nil
		}

		for sourceRows.Next() {
			scanValues := make([]interface{}, len(columns))
			scanTargets := make([]interface{}, len(columns))
			for i := range scanValues {
				scanTargets[i] = &scanValues[i]
			}
			if err := sourceRows.Scan(scanTargets...); err != nil {
				return fmt.Errorf("scan source row failed: %w", err)
			}

			valueClauses = append(valueClauses, rowPlaceholder)
			for _, value := range scanValues {
				batchArgs = append(batchArgs, normalizeValue(value))
			}
			returnCount++

			if len(valueClauses) >= batchSize {
				if err := flush(); err != nil {
					return fmt.Errorf("flush insert batch failed: %w", err)
				}
			}
		}
		if err := sourceRows.Err(); err != nil {
			return fmt.Errorf("iterate source rows failed: %w", err)
		}
		if err := flush(); err != nil {
			return fmt.Errorf("flush final insert batch failed: %w", err)
		}

		var targetCount int64
		if err := tx.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(table))).Scan(&targetCount).Error; err != nil {
			return fmt.Errorf("count target rows failed: %w", err)
		}
		if int(targetCount) != returnCount {
			return fmt.Errorf("row count mismatch: source=%d target=%d", returnCount, targetCount)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return returnCount, nil
}

func joinQuoted(columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, quoteIdent(column))
	}
	return strings.Join(quoted, ",")
}

func quoteIdent(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func normalizeValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case []byte:
		return append([]byte(nil), typed...)
	case string:
		return typed
	case time.Time:
		return typed
	default:
		return value
	}
}
