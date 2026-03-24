package core

// PlatformCapabilities describes behavior that higher-level bridge components
// can use without hard-coding platform names.
type PlatformCapabilities struct {
	SupportsRichMessages   bool
	SupportsDeferredReply  bool
	RequiresPublicCallback bool
	SupportsSlashCommands  bool
	SupportsMentions       bool
}

// PlatformMetadata captures the stable source identity and declared
// capabilities of a platform implementation.
type PlatformMetadata struct {
	Source       string
	Capabilities PlatformCapabilities
}

// MetadataProvider is an optional interface for platforms that can declare
// metadata explicitly.
type MetadataProvider interface {
	Metadata() PlatformMetadata
}

// MetadataForPlatform returns normalized metadata for a platform. Platforms can
// override the defaults by implementing MetadataProvider.
func MetadataForPlatform(platform Platform) PlatformMetadata {
	defaults := PlatformMetadata{
		Source: NormalizePlatformName(platform.Name()),
	}
	if _, ok := platform.(CardSender); ok {
		defaults.Capabilities.SupportsRichMessages = true
	}

	provider, ok := platform.(MetadataProvider)
	if !ok {
		return defaults
	}

	metadata := provider.Metadata()
	if normalized := NormalizePlatformName(metadata.Source); normalized != "" {
		metadata.Source = normalized
	} else {
		metadata.Source = defaults.Source
	}
	return metadata
}
