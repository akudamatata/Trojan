package core

import (
	"fmt"
	"os/exec"
	"strings"
)

// FirewallType represents the detected firewall system
type FirewallType string

const (
	FirewallIPTables  FirewallType = "iptables"
	FirewallUFW       FirewallType = "ufw"
	FirewallFirewalld FirewallType = "firewalld"
	FirewallNone      FirewallType = "none"
)

var activeFirewall FirewallType = FirewallNone

func DetectFirewall() FirewallType {
	if activeFirewall != FirewallNone {
		return activeFirewall
	}

	// 1. Check if ufw is active
	if err := exec.Command("which", "ufw").Run(); err == nil {
		out, err := exec.Command("ufw", "status").Output()
		if err == nil && strings.Contains(string(out), "Status: active") {
			activeFirewall = FirewallUFW
			return activeFirewall
		}
	}

	// 2. Check if firewalld is active
	if err := exec.Command("which", "firewall-cmd").Run(); err == nil {
		out, err := exec.Command("firewall-cmd", "--state").Output()
		if err == nil && strings.TrimSpace(string(out)) == "running" {
			activeFirewall = FirewallFirewalld
			return activeFirewall
		}
	}

	// 3. Fallback to iptables
	if err := exec.Command("which", "iptables").Run(); err == nil {
		activeFirewall = FirewallIPTables
		return activeFirewall
	}

	activeFirewall = FirewallNone
	return activeFirewall
}

// BlockIP blocks all traffic from a client IP address
func BlockIP(ip string) error {
	fw := DetectFirewall()
	switch fw {
	case FirewallUFW:
		cmd := exec.Command("ufw", "insert", "1", "deny", "from", ip)
		return cmd.Run()
	case FirewallFirewalld:
		family := "ipv4"
		if strings.Contains(ip, ":") {
			family = "ipv6"
		}
		rule := fmt.Sprintf("rule family='%s' source address='%s' drop", family, ip)
		cmd := exec.Command("firewall-cmd", "--add-rich-rule="+rule)
		if err := cmd.Run(); err != nil {
			return err
		}
		exec.Command("firewall-cmd", "--permanent", "--add-rich-rule="+rule).Run()
		return nil
	case FirewallIPTables:
		iptablesCmd := "iptables"
		if strings.Contains(ip, ":") {
			iptablesCmd = "ip6tables"
		}
		cmd := exec.Command(iptablesCmd, "-I", "INPUT", "-s", ip, "-j", "DROP")
		if err := cmd.Run(); err != nil {
			return err
		}
		exec.Command("service", "iptables", "save").Run()
		exec.Command("service", "ip6tables", "save").Run()
		return nil
	default:
		iptablesCmd := "iptables"
		if strings.Contains(ip, ":") {
			iptablesCmd = "ip6tables"
		}
		return exec.Command(iptablesCmd, "-I", "INPUT", "-s", ip, "-j", "DROP").Run()
	}
}

// UnblockIP unblocks a client IP address
func UnblockIP(ip string) error {
	fw := DetectFirewall()
	switch fw {
	case FirewallUFW:
		cmd := exec.Command("ufw", "delete", "deny", "from", ip)
		return cmd.Run()
	case FirewallFirewalld:
		family := "ipv4"
		if strings.Contains(ip, ":") {
			family = "ipv6"
		}
		rule := fmt.Sprintf("rule family='%s' source address='%s' drop", family, ip)
		cmd := exec.Command("firewall-cmd", "--remove-rich-rule="+rule)
		if err := cmd.Run(); err != nil {
			return err
		}
		exec.Command("firewall-cmd", "--permanent", "--remove-rich-rule="+rule).Run()
		return nil
	case FirewallIPTables:
		iptablesCmd := "iptables"
		if strings.Contains(ip, ":") {
			iptablesCmd = "ip6tables"
		}
		for {
			cmd := exec.Command(iptablesCmd, "-D", "INPUT", "-s", ip, "-j", "DROP")
			if err := cmd.Run(); err != nil {
				break
			}
		}
		exec.Command("service", "iptables", "save").Run()
		exec.Command("service", "ip6tables", "save").Run()
		return nil
	default:
		iptablesCmd := "iptables"
		if strings.Contains(ip, ":") {
			iptablesCmd = "ip6tables"
		}
		for {
			cmd := exec.Command(iptablesCmd, "-D", "INPUT", "-s", ip, "-j", "DROP")
			if err := cmd.Run(); err != nil {
				break
			}
		}
		return nil
	}
}

// SyncBlacklist restores all active IP blocks from MySQL to the firewall on startup
func SyncBlacklist() {
	mysql := GetMysql()
	db := mysql.GetDB()
	if db == nil {
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT ip FROM ip_blacklist WHERE expire_at IS NULL OR expire_at > NOW()")
	if err != nil {
		fmt.Println("[Firewall] Sync error:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err == nil {
			UnblockIP(ip)
			if err := BlockIP(ip); err != nil {
				fmt.Printf("[Firewall] Failed to block IP %s during sync: %v\n", ip, err)
			} else {
				fmt.Printf("[Firewall] Successfully synced block for IP: %s\n", ip)
			}
		}
	}
}

// CleanExpiredBlacklist checks for expired IP bans, unbans them in the firewall, and removes them from MySQL
func CleanExpiredBlacklist() {
	mysql := GetMysql()
	db := mysql.GetDB()
	if db == nil {
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT ip FROM ip_blacklist WHERE expire_at IS NOT NULL AND expire_at <= NOW()")
	if err != nil {
		return
	}
	defer rows.Close()

	expiredIPs := []string{}
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err == nil {
			expiredIPs = append(expiredIPs, ip)
		}
	}

	for _, ip := range expiredIPs {
		if err := UnblockIP(ip); err == nil {
			_, _ = db.Exec("DELETE FROM ip_blacklist WHERE ip = ?", ip)
			fmt.Printf("[Firewall] Expired ban cleared for IP: %s\n", ip)
		} else {
			fmt.Printf("[Firewall] Failed to unblock expired IP %s: %v\n", ip, err)
		}
	}
}
