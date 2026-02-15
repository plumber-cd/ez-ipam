package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/plumber-cd/ez-ipam/internal/domain"
	"sigs.k8s.io/yaml"
)

const (
	DataDirName      = ".ez-ipam"
	MarkdownFileName = "EZ-IPAM.md"

	networksDirName  = "networks"
	ipsDirName       = "ips"
	vlansDirName     = "vlans"
	ssidsDirName     = "ssids"
	zonesDirName     = "zones"
	equipmentDirName = "equipment"
	portsDirName     = "ports"
)

// Load reads all YAML files from the data directory and returns a populated Catalog.
func Load(dir string) (*domain.Catalog, error) {
	catalog := domain.NewCatalog()

	// Add static folders.
	catalog.Put(&domain.StaticFolder{
		Base:        domain.Base{ID: domain.FolderNetworks},
		Index:       0,
		Description: "Manage your address space here.\n\nUse Enter or double-click to open items.\nUse Backspace to go up one level.",
	})
	catalog.Put(&domain.StaticFolder{
		Base:        domain.Base{ID: domain.FolderZones},
		Index:       1,
		Description: "Document network security zones and associated VLANs.",
	})
	catalog.Put(&domain.StaticFolder{
		Base:        domain.Base{ID: domain.FolderVLANs},
		Index:       2,
		Description: "Manage VLAN IDs and their metadata here.",
	})
	catalog.Put(&domain.StaticFolder{
		Base:        domain.Base{ID: domain.FolderSSIDs},
		Index:       3,
		Description: "Manage WiFi SSIDs and their metadata here.",
	})
	catalog.Put(&domain.StaticFolder{
		Base:        domain.Base{ID: domain.FolderEquipment},
		Index:       4,
		Description: "Track network equipment, ports, VLAN profiles, and links.",
	})

	dataDir := filepath.Join(dir, DataDirName)
	var loadErr error
	loadDir := func(subDir string, create func([]byte) (domain.Item, error)) {
		if loadErr != nil {
			return
		}
		fullPath := filepath.Join(dataDir, subDir)
		files, err := os.ReadDir(fullPath)
		if err != nil {
			if !os.IsNotExist(err) {
				loadErr = fmt.Errorf("read %s directory: %w", fullPath, err)
				return
			}
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				loadErr = fmt.Errorf("create %s directory: %w", fullPath, err)
				return
			}
			return
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			bytes, err := os.ReadFile(filepath.Join(fullPath, f.Name()))
			if err != nil {
				loadErr = fmt.Errorf("read %s: %w", f.Name(), err)
				return
			}
			item, err := create(bytes)
			if err != nil {
				loadErr = fmt.Errorf("unmarshal %s: %w", f.Name(), err)
				return
			}
			catalog.Put(item)
		}
	}

	loadDir(networksDirName, func(bytes []byte) (domain.Item, error) {
		n := &domain.Network{}
		if err := yaml.Unmarshal(bytes, n); err != nil {
			return nil, err
		}
		return n, nil
	})

	loadDir(ipsDirName, func(bytes []byte) (domain.Item, error) {
		ip := &domain.IP{}
		if err := yaml.Unmarshal(bytes, ip); err != nil {
			return nil, err
		}
		return ip, nil
	})

	loadDir(vlansDirName, func(bytes []byte) (domain.Item, error) {
		v := &domain.VLAN{}
		if err := yaml.Unmarshal(bytes, v); err != nil {
			return nil, err
		}
		return v, nil
	})

	loadDir(ssidsDirName, func(bytes []byte) (domain.Item, error) {
		s := &domain.SSID{}
		if err := yaml.Unmarshal(bytes, s); err != nil {
			return nil, err
		}
		return s, nil
	})

	loadDir(zonesDirName, func(bytes []byte) (domain.Item, error) {
		z := &domain.Zone{}
		if err := yaml.Unmarshal(bytes, z); err != nil {
			return nil, err
		}
		return z, nil
	})

	loadDir(equipmentDirName, func(bytes []byte) (domain.Item, error) {
		e := &domain.Equipment{}
		if err := yaml.Unmarshal(bytes, e); err != nil {
			return nil, err
		}
		return e, nil
	})

	loadDir(portsDirName, func(bytes []byte) (domain.Item, error) {
		p := &domain.Port{}
		if err := yaml.Unmarshal(bytes, p); err != nil {
			return nil, err
		}
		return p, nil
	})

	if loadErr != nil {
		return nil, loadErr
	}

	// Normalize and validate all loaded items.
	for _, item := range catalog.All() {
		if z, ok := item.(*domain.Zone); ok {
			z.Normalize()
		}
		if err := item.Validate(catalog); err != nil {
			return nil, fmt.Errorf("validate %s: %w", item.GetPath(), err)
		}
	}

	return catalog, nil
}

// Save writes all catalog items to YAML files using an atomic rename.
func Save(dir string, catalog *domain.Catalog) error {
	dataDir := filepath.Join(dir, DataDirName)
	dataTmpDir := dataDir + ".tmp"
	dataOldDir := dataDir + ".old"

	networksTmpDir := filepath.Join(dataTmpDir, networksDirName)
	ipsTmpDir := filepath.Join(dataTmpDir, ipsDirName)
	vlansTmpDir := filepath.Join(dataTmpDir, vlansDirName)
	ssidsTmpDir := filepath.Join(dataTmpDir, ssidsDirName)
	zonesTmpDir := filepath.Join(dataTmpDir, zonesDirName)
	equipmentTmpDir := filepath.Join(dataTmpDir, equipmentDirName)
	portsTmpDir := filepath.Join(dataTmpDir, portsDirName)

	if err := os.RemoveAll(dataTmpDir); err != nil {
		return fmt.Errorf("remove tmp dir: %w", err)
	}
	for _, d := range []string{networksTmpDir, ipsTmpDir, vlansTmpDir, ssidsTmpDir, zonesTmpDir, equipmentTmpDir, portsTmpDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}

	for _, item := range catalog.All() {
		switch m := item.(type) {
		case *domain.StaticFolder:
			// Not serializable.
		case *domain.Network:
			id, err := domain.CIDRToIdentifier(m.ID)
			if err != nil {
				return fmt.Errorf("convert %s to identifier: %w", m.ID, err)
			}
			if err := writeYAML(filepath.Join(networksTmpDir, id+".yaml"), m); err != nil {
				return err
			}
		case *domain.IP:
			id, err := domain.IPToIdentifier(m.ID)
			if err != nil {
				return fmt.Errorf("convert %s to identifier: %w", m.ID, err)
			}
			if err := writeYAML(filepath.Join(ipsTmpDir, id+".yaml"), m); err != nil {
				return err
			}
		case *domain.VLAN:
			if err := writeYAML(filepath.Join(vlansTmpDir, m.ID+".yaml"), m); err != nil {
				return err
			}
		case *domain.SSID:
			if err := writeYAML(filepath.Join(ssidsTmpDir, m.ID+".yaml"), m); err != nil {
				return err
			}
		case *domain.Zone:
			if err := writeYAML(filepath.Join(zonesTmpDir, safeFileNameSegment(m.ID)+".yaml"), m); err != nil {
				return err
			}
		case *domain.Equipment:
			if err := writeYAML(filepath.Join(equipmentTmpDir, safeFileNameSegment(m.ID)+".yaml"), m); err != nil {
				return err
			}
		case *domain.Port:
			parent := catalog.Get(m.GetParentPath())
			if parent == nil {
				return fmt.Errorf("port parent not found for %s", m.GetPath())
			}
			parentEquipment, ok := parent.(*domain.Equipment)
			if !ok {
				return fmt.Errorf("port parent is not equipment for %s", m.GetPath())
			}
			if err := writeYAML(filepath.Join(portsTmpDir, safeFileNameSegment(parentEquipment.ID)+"_"+m.ID+".yaml"), m); err != nil {
				return err
			}
		}
	}

	if err := os.RemoveAll(dataOldDir); err != nil {
		return fmt.Errorf("remove old dir: %w", err)
	}
	if _, err := os.Stat(dataDir); err == nil {
		if err := os.Rename(dataDir, dataOldDir); err != nil {
			return fmt.Errorf("rename %s to %s: %w", dataDir, dataOldDir, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dataDir, err)
	}
	if err := os.Rename(dataTmpDir, dataDir); err != nil {
		// best-effort rollback
		if _, rollbackErr := os.Stat(dataOldDir); rollbackErr == nil {
			_ = os.Rename(dataOldDir, dataDir)
		}
		return fmt.Errorf("rename %s to %s: %w", dataTmpDir, dataDir, err)
	}
	if err := os.RemoveAll(dataOldDir); err != nil {
		return fmt.Errorf("remove old dir after swap: %w", err)
	}
	return nil
}

func writeYAML(fileName string, v interface{}) error {
	bytes, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal %T: %w", v, err)
	}
	if err := os.WriteFile(fileName, bytes, 0644); err != nil {
		return fmt.Errorf("write %s: %w", fileName, err)
	}
	return nil
}

// safeFileNameSegment sanitizes a string for use as a filename.
func safeFileNameSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "item"
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, trimmed)
	return strings.Trim(safe, "_")
}
