package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"slices"
	"strings"

	gopast "github.com/goplus/gop/ast"
	"github.com/goplus/goxlsw/internal/vfs"
)

// SpxResourceID is the ID of an spx resource.
type SpxResourceID interface {
	Name() string
	URI() SpxResourceURI
}

// SpxResourceRef is a reference to an spx resource.
type SpxResourceRef struct {
	ID   SpxResourceID
	Kind SpxResourceRefKind
	Node gopast.Node
}

// SpxResourceRefKind is the kind of an spx resource reference.
type SpxResourceRefKind string

const (
	SpxResourceRefKindStringLiteral        SpxResourceRefKind = "stringLiteral"
	SpxResourceRefKindAutoBinding          SpxResourceRefKind = "autoBinding"
	SpxResourceRefKindAutoBindingReference SpxResourceRefKind = "autoBindingReference"
	SpxResourceRefKindConstantReference    SpxResourceRefKind = "constantReference"
)

// ParseSpxResourceURI parses an spx resource URI and returns the corresponding
// spx resource ID.
func ParseSpxResourceURI(uri SpxResourceURI) (SpxResourceID, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to parse spx resource URI: %w", err)
	}
	pathParts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	pathPartCount := len(pathParts)
	if u.Scheme != "spx" || u.Host != "resources" || path.Clean(u.Path) != u.Path || pathPartCount < 2 {
		return nil, fmt.Errorf("invalid spx resource URI: %s", uri)
	}
	switch pathParts[0] {
	case "backdrops":
		return SpxBackdropResourceID{BackdropName: pathParts[1]}, nil
	case "sounds":
		return SpxSoundResourceID{SoundName: pathParts[1]}, nil
	case "sprites":
		if pathPartCount == 2 {
			return SpxSpriteResourceID{SpriteName: pathParts[1]}, nil
		}
		if pathPartCount > 3 {
			switch pathParts[2] {
			case "costumes":
				return SpxSpriteCostumeResourceID{SpriteName: pathParts[1], CostumeName: pathParts[3]}, nil
			case "animations":
				return SpxSpriteAnimationResourceID{SpriteName: pathParts[1], AnimationName: pathParts[3]}, nil
			}
		}
	case "widgets":
		return SpxWidgetResourceID{WidgetName: pathParts[1]}, nil
	}
	return nil, fmt.Errorf("unsupported or malformed spx resource type in URI: %s", uri)
}

// SpxResourceSet is a set of spx resources.
type SpxResourceSet struct {
	backdrops map[string]*SpxBackdropResource
	sounds    map[string]*SpxSoundResource
	sprites   map[string]*SpxSpriteResource
	widgets   map[string]*SpxWidgetResource
}

// NewSpxResourceSet creates a new spx resource set.
func NewSpxResourceSet(rootFS vfs.SubFS) (*SpxResourceSet, error) {
	set := &SpxResourceSet{
		backdrops: make(map[string]*SpxBackdropResource),
		sounds:    make(map[string]*SpxSoundResource),
		sprites:   make(map[string]*SpxSpriteResource),
		widgets:   make(map[string]*SpxWidgetResource),
	}

	// Read and parse the main index.json for backdrops and widgets.
	metadata, err := rootFS.ReadFile("index.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read index.json: %w", err)
	}

	var assets struct {
		Backdrops []SpxBackdropResource `json:"backdrops"`
		Zorder    []json.RawMessage     `json:"zorder"`
	}
	if err := json.Unmarshal(metadata, &assets); err != nil {
		return nil, fmt.Errorf("failed to parse index.json: %w", err)
	}

	// Process backdrops.
	for _, backdrop := range assets.Backdrops {
		backdrop.ID = SpxBackdropResourceID{BackdropName: backdrop.Name}
		set.backdrops[backdrop.Name] = &backdrop
	}

	// Process widgets from zorder.
	for _, item := range assets.Zorder {
		var widget SpxWidgetResource
		if err := json.Unmarshal(item, &widget); err == nil && widget.Name != "" {
			widget.ID = SpxWidgetResourceID{WidgetName: widget.Name}
			set.widgets[widget.Name] = &widget
		}
	}

	// Read sounds directory.
	soundEntries, err := rootFS.Readdir("sounds")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to read sounds directory: %w", err)
	}
	for _, entry := range soundEntries {
		if !entry.IsDir() {
			continue
		}

		soundName := entry.Name()
		soundMetadata, err := rootFS.ReadFile(path.Join("sounds", soundName, "index.json"))
		if err != nil {
			return nil, fmt.Errorf("failed to read sound metadata: %w", err)
		}

		var sound SpxSoundResource
		if err := json.Unmarshal(soundMetadata, &sound); err != nil {
			return nil, fmt.Errorf("failed to parse sound metadata: %w", err)
		}
		sound.Name = soundName
		sound.ID = SpxSoundResourceID{SoundName: soundName}
		set.sounds[soundName] = &sound
	}

	// Read sprites directory.
	spriteEntries, err := rootFS.Readdir("sprites")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to read sprites directory: %w", err)
	}
	for _, entry := range spriteEntries {
		if !entry.IsDir() {
			continue
		}

		spriteName := entry.Name()
		spriteMetadata, err := rootFS.ReadFile(path.Join("sprites", spriteName, "index.json"))
		if err != nil {
			return nil, fmt.Errorf("failed to read sprite metadata: %w", err)
		}

		sprite := SpxSpriteResource{
			ID:   SpxSpriteResourceID{SpriteName: spriteName},
			Name: spriteName,
		}
		if err := json.Unmarshal(spriteMetadata, &sprite); err != nil {
			return nil, fmt.Errorf("failed to parse sprite metadata: %w", err)
		}

		// Process costumes.
		for i, costume := range sprite.Costumes {
			sprite.Costumes[i].ID = SpxSpriteCostumeResourceID{
				SpriteName:  spriteName,
				CostumeName: costume.Name,
			}
		}

		// Process animations.
		sprite.Animations = make([]SpxSpriteAnimationResource, 0, len(sprite.FAnimations))
		for animName, fAnim := range sprite.FAnimations {
			sprite.Animations = append(sprite.Animations, SpxSpriteAnimationResource{
				ID:        SpxSpriteAnimationResourceID{SpriteName: spriteName, AnimationName: animName},
				Name:      animName,
				FromIndex: getCostumeIndex(fAnim.FrameFrom, sprite.Costumes),
				ToIndex:   getCostumeIndex(fAnim.FrameTo, sprite.Costumes),
			})
		}

		// Process normal costumes.
		sprite.NormalCostumes = make([]SpxSpriteCostumeResource, 0, len(sprite.Costumes))
		for i, costume := range sprite.Costumes {
			isAnimation := slices.ContainsFunc(sprite.Animations, func(anim SpxSpriteAnimationResource) bool {
				return anim.includeCostume(i)
			})
			if !isAnimation {
				sprite.NormalCostumes = append(sprite.NormalCostumes, costume)
			}
		}

		set.sprites[spriteName] = &sprite
	}

	return set, nil
}

// Backdrop returns the backdrop with the given name. It returns nil if not found.
func (set *SpxResourceSet) Backdrop(name string) *SpxBackdropResource {
	if set.backdrops == nil {
		return nil
	}
	return set.backdrops[name]
}

// Sound returns the sound with the given name. It returns nil if not found.
func (set *SpxResourceSet) Sound(name string) *SpxSoundResource {
	if set.sounds == nil {
		return nil
	}
	return set.sounds[name]
}

// Sprite returns the sprite with the given name. It returns nil if not found.
func (set *SpxResourceSet) Sprite(name string) *SpxSpriteResource {
	if set.sprites == nil {
		return nil
	}
	return set.sprites[name]
}

// Widget returns the widget with the given name. It returns nil if not found.
func (set *SpxResourceSet) Widget(name string) *SpxWidgetResource {
	if set.widgets == nil {
		return nil
	}
	return set.widgets[name]
}

// SpxBackdropResource represents a backdrop resource in spx.
type SpxBackdropResource struct {
	ID   SpxBackdropResourceID `json:"-"`
	Name string                `json:"name"`
	Path string                `json:"path"`
}

// SpxBackdropResourceID is the ID of an spx backdrop resource.
type SpxBackdropResourceID struct {
	BackdropName string
}

// Name implements [SpxResourceID].
func (id SpxBackdropResourceID) Name() string {
	return id.BackdropName
}

// URI implements [SpxResourceID].
func (id SpxBackdropResourceID) URI() SpxResourceURI {
	return SpxResourceURI(fmt.Sprintf("spx://resources/backdrops/%s", id.BackdropName))
}

// SpxSoundResource represents a sound resource in spx.
type SpxSoundResource struct {
	ID   SpxSoundResourceID `json:"-"`
	Name string             `json:"name"`
	Path string             `json:"path"`
}

// SpxSoundResourceID is the ID of an spx sound resource.
type SpxSoundResourceID struct {
	SoundName string
}

// Name implements [SpxResourceID].
func (id SpxSoundResourceID) Name() string {
	return id.SoundName
}

// URI implements [SpxResourceID].
func (id SpxSoundResourceID) URI() SpxResourceURI {
	return SpxResourceURI(fmt.Sprintf("spx://resources/sounds/%s", id.SoundName))
}

type spxSpriteFAnimation struct {
	FrameFrom string `json:"frameFrom"`
	FrameTo   string `json:"frameTo"`
}

// SpxSpriteResource represents an spx sprite resource.
type SpxSpriteResource struct {
	ID       SpxSpriteResourceID        `json:"-"`
	Name     string                     `json:"name"`
	Costumes []SpxSpriteCostumeResource `json:"costumes"`
	// NormalCostumes includes all costumes except animation costumes.
	NormalCostumes   []SpxSpriteCostumeResource     `json:"-"`
	CostumeIndex     int                            `json:"costumeIndex"`
	FAnimations      map[string]spxSpriteFAnimation `json:"fAnimations"`
	Animations       []SpxSpriteAnimationResource   `json:"-"`
	DefaultAnimation string                         `json:"defaultAnimation"`
}

// SpxSpriteResourceID is the ID of an spx sprite resource.
type SpxSpriteResourceID struct {
	SpriteName string
}

// Name implements [SpxResourceID].
func (id SpxSpriteResourceID) Name() string {
	return id.SpriteName
}

// URI implements [SpxResourceID].
func (id SpxSpriteResourceID) URI() SpxResourceURI {
	return SpxResourceURI(fmt.Sprintf("spx://resources/sprites/%s", id.SpriteName))
}

// Costume returns the costume with the given name. It returns nil if not found.
func (sprite *SpxSpriteResource) Costume(name string) *SpxSpriteCostumeResource {
	idx := slices.IndexFunc(sprite.Costumes, func(costume SpxSpriteCostumeResource) bool {
		return costume.Name == name
	})
	if idx < 0 {
		return nil
	}
	return &sprite.Costumes[idx]
}

// Animation returns the animation with the given name. It returns nil if not found.
func (sprite *SpxSpriteResource) Animation(name string) *SpxSpriteAnimationResource {
	idx := slices.IndexFunc(sprite.Animations, func(animation SpxSpriteAnimationResource) bool {
		return animation.Name == name
	})
	if idx < 0 {
		return nil
	}
	return &sprite.Animations[idx]
}

// SpxSpriteCostumeResource represents an spx sprite costume resource.
type SpxSpriteCostumeResource struct {
	ID   SpxSpriteCostumeResourceID `json:"-"`
	Name string                     `json:"name"`
	Path string                     `json:"path"`
}

// SpxSpriteCostumeResourceID is the ID of an spx sprite costume resource.
type SpxSpriteCostumeResourceID struct {
	SpriteName  string
	CostumeName string
}

// Name implements [SpxResourceID].
func (id SpxSpriteCostumeResourceID) Name() string {
	return id.CostumeName
}

// URI implements [SpxResourceID].
func (id SpxSpriteCostumeResourceID) URI() SpxResourceURI {
	return SpxResourceURI(fmt.Sprintf("spx://resources/sprites/%s/costumes/%s", id.SpriteName, id.CostumeName))
}

// SpxSpriteAnimationResource represents an spx sprite animation resource.
type SpxSpriteAnimationResource struct {
	ID        SpxSpriteAnimationResourceID `json:"-"`
	Name      string                       `json:"name"`
	FromIndex *int                         `json:"-"`
	ToIndex   *int                         `json:"-"`
}

func (a *SpxSpriteAnimationResource) includeCostume(index int) bool {
	if a.FromIndex == nil || a.ToIndex == nil {
		return false
	}
	return *a.FromIndex <= index && index <= *a.ToIndex
}

// SpxSpriteAnimationResourceID is the ID of an spx sprite animation resource.
type SpxSpriteAnimationResourceID struct {
	SpriteName    string
	AnimationName string
}

// Name implements [SpxResourceID].
func (id SpxSpriteAnimationResourceID) Name() string {
	return id.AnimationName
}

// URI implements [SpxResourceID].
func (id SpxSpriteAnimationResourceID) URI() SpxResourceURI {
	return SpxResourceURI(fmt.Sprintf("spx://resources/sprites/%s/animations/%s", id.SpriteName, id.AnimationName))
}

// SpxWidgetResource represents a widget resource in spx.
type SpxWidgetResource struct {
	ID    SpxWidgetResourceID `json:"-"`
	Name  string              `json:"name"`
	Type  string              `json:"type"`
	Label string              `json:"label"`
	Val   string              `json:"val"`
}

// SpxWidgetResourceID is the ID of an spx widget resource.
type SpxWidgetResourceID struct {
	WidgetName string
}

// Name implements [SpxResourceID].
func (id SpxWidgetResourceID) Name() string {
	return id.WidgetName
}

// URI implements [SpxResourceID].
func (id SpxWidgetResourceID) URI() SpxResourceURI {
	return SpxResourceURI(fmt.Sprintf("spx://resources/widgets/%s", id.WidgetName))
}

func getCostumeIndex(name string, costumes []SpxSpriteCostumeResource) *int {
	for i, costume := range costumes {
		if costume.Name == name {
			return &i
		}
	}
	return nil
}
