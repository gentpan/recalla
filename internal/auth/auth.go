package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/gentpan/recalla/internal/db"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Service 用户认证服务
type Service struct {
	db *db.DB
}

// User 用户
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKey API 密钥
type APIKey struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// NewService 创建认证服务
func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

// EnsureAdmin 确保默认管理员存在
func (s *Service) EnsureAdmin(ctx context.Context) error {
	var count int
	s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if count > 0 {
		return nil
	}

	// 创建默认管理员
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO users (id, username, password, role, created_at, updated_at)
		VALUES ($1, $2, $3, 'admin', $4, $4)
	`, uuid.New().String(), "admin", string(hash), time.Now())
	if err != nil {
		return fmt.Errorf("创建默认管理员失败: %w", err)
	}
	log.Println("已创建默认管理员: admin / admin123 （请尽快修改密码）")
	return nil
}

// Login 登录验证，返回用户信息
func (s *Service) Login(ctx context.Context, username, password string) (*User, error) {
	var user User
	var hash string
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id, username, role, password, created_at FROM users WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &user.Role, &hash, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("密码错误")
	}
	return &user, nil
}

// GenerateToken 生成会话 token（简单 HMAC 签名）
func (s *Service) GenerateToken(userID, username, role string) string {
	data := fmt.Sprintf("%s|%s|%s|%d", userID, username, role, time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("recalla-secret-key-2026"))
	mac.Write([]byte(data))
	sig := hex.EncodeToString(mac.Sum(nil))
	return hex.EncodeToString([]byte(data)) + "." + sig
}

// ValidateToken 验证 token，返回用户信息
func (s *Service) ValidateToken(token string) (*User, error) {
	parts := splitToken(token)
	if len(parts) != 2 {
		return nil, fmt.Errorf("token 格式错误")
	}

	data, err := hex.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("token 解码失败")
	}

	// 验证签名
	mac := hmac.New(sha256.New, []byte("recalla-secret-key-2026"))
	mac.Write(data)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return nil, fmt.Errorf("token 签名无效")
	}

	// 解析数据
	var userID, username, role string
	var ts int64
	_, err = fmt.Sscanf(string(data), "%s", &userID)
	if err != nil {
		return nil, fmt.Errorf("token 数据错误")
	}

	// 简单解析 pipe 分隔
	fields := splitPipe(string(data))
	if len(fields) < 4 {
		return nil, fmt.Errorf("token 数据不完整")
	}
	userID = fields[0]
	username = fields[1]
	role = fields[2]
	fmt.Sscanf(fields[3], "%d", &ts)

	// 检查过期（7 天）
	if time.Now().Unix()-ts > 7*24*3600 {
		return nil, fmt.Errorf("token 已过期")
	}

	return &User{ID: userID, Username: username, Role: role}, nil
}

// ChangePassword 修改密码
func (s *Service) ChangePassword(ctx context.Context, userID, oldPwd, newPwd string) error {
	var hash string
	err := s.db.Pool.QueryRow(ctx, `SELECT password FROM users WHERE id = $1`, userID).Scan(&hash)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPwd)); err != nil {
		return fmt.Errorf("旧密码错误")
	}

	newHash, _ := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	_, err = s.db.Pool.Exec(ctx, `UPDATE users SET password=$1, updated_at=$2 WHERE id=$3`, string(newHash), time.Now(), userID)
	return err
}

// ChangeUsername 修改用户名
func (s *Service) ChangeUsername(ctx context.Context, userID, newUsername string) error {
	_, err := s.db.Pool.Exec(ctx, `UPDATE users SET username=$1, updated_at=$2 WHERE id=$3`, newUsername, time.Now(), userID)
	return err
}

// GenerateAPIKey 生成新的 API Key
func (s *Service) GenerateAPIKey(ctx context.Context, userID, name string) (*APIKey, error) {
	key := "rk-" + randomHex(24)
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, key, name, created_at) VALUES ($1, $2, $3, $4, $5)
	`, id, userID, key, name, now)
	if err != nil {
		return nil, fmt.Errorf("生成 API Key 失败: %w", err)
	}

	return &APIKey{ID: id, UserID: userID, Key: key, Name: name, CreatedAt: now}, nil
}

// ListAPIKeys 列出用户的 API Key
func (s *Service) ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, user_id, key, name, last_used, created_at FROM api_keys WHERE user_id=$1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Key, &k.Name, &k.LastUsed, &k.CreatedAt); err == nil {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// DeleteAPIKey 删除 API Key
func (s *Service) DeleteAPIKey(ctx context.Context, userID, keyID string) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM api_keys WHERE id=$1 AND user_id=$2`, keyID, userID)
	return err
}

// ValidateAPIKey 验证 API Key，返回用户 ID
func (s *Service) ValidateAPIKey(ctx context.Context, key string) (string, error) {
	var userID string
	err := s.db.Pool.QueryRow(ctx, `SELECT user_id FROM api_keys WHERE key=$1`, key).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("API Key 无效")
	}
	// 更新最后使用时间
	go func() {
		s.db.Pool.Exec(context.Background(), `UPDATE api_keys SET last_used=$1 WHERE key=$2`, time.Now(), key)
	}()
	return userID, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func splitToken(s string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}

func splitPipe(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}
