# 3D Model Procurement & Conversion Log

## Asset Sources

| Model | Source | License | URL | Format |
|-------|--------|---------|-----|--------|
| RPG Characters (Cleric, Monk, Ranger, Rogue, Warrior, Wizard) | Quaternius / OpenGameArt | CC0 | https://opengameart.org/content/lowpoly-rpg-characters | FBX, OBJ, Blend |
| Textured LowPoly Trees (Tree_1, Tree_5, Pine_1, Birch_1, DeadTree_1) | Quaternius / OpenGameArt | CC0 | https://opengameart.org/content/textured-lowpoly-trees | OBJ, MTL, PNG |
| Ultimate Buildings Pack (Dec 2019) | Quaternius / OpenGameArt | CC0 | https://opengameart.org/content/ultimate-buildings-pack | OBJ, MTL, PNG |
| Ultimate Nature Pack (Jun 2019) | Quaternius / OpenGameArt | CC0 | https://opengameart.org/content/ultimate-nature-pack | OBJ, MTL, PNG |
| Medieval Village Props (Well, Bonfire, MarketStand, Cart, Cauldron) | Quaternius / OpenGameArt | CC0 | https://opengameart.org/content/medieval-village | OBJ, MTL, PNG |
| Grass Texture | GrEmlin / OpenGameArt | CC0 | https://opengameart.org/content/seamless-grass-texture-grass55png | PNG 512x512 |
| Plasma Water Texture | procedural | — | generated in script | PNG 128x128 |

## Conversion Pipeline (FBX → glTF 2.0 → .skl)

The Quaternius RPG pack ships in FBX/Blend/OBJ formats. FBX cannot be parsed natively in `za`, so an offline two-stage conversion is required:

1. **FBX → glTF 2.0** via `fbx2gltf` (Facebook's converter)
2. **glTF 2.0 → .skl** via `./tools/process_gltf.py` (custom Python preprocessor)

### Prerequisites

- `npm` (Node.js package manager)
- `fbx2gltf` npm package (Facebook's FBX→glTF converter)
- Python 3

### Installation

```bash
cd /tmp
npm install fbx2gltf
```

### Stage 1: FBX → glTF Conversion

```bash
FBX2glTF="/tmp/node_modules/fbx2gltf/bin/Linux/FBX2glTF"
chmod +x "$FBX2glTF"

for char in Cleric Monk Ranger Rogue Warrior Wizard; do
    "$FBX2glTF" -i "${char}.fbx" -o "${char}_out/${char}"
done
```

Each conversion produces:
- `{Character}_out/{Character}.gltf` — JSON manifest (nodes, skins, animations, accessors)
- `{Character}_out/buffer.bin` — Binary vertex data, indices, inverse bind matrices, animation keyframes

### Stage 2: glTF → .skl Binary

```bash
cd ~/3d
python3 process_gltf.py assets/models/characters/*_out/*.gltf
```

The `process_gltf.py` script reads the glTF JSON + buffer.bin and writes a custom `.skl` binary format for fast loading by `libskel.so`.

### .skl Binary Format (SKL1)

```
Header:    magic(4) num_verts(4) num_indices(4) num_joints(4) num_anims(4) max_kf(4) num_nodes(4)
Mesh:      positions[verts*3] normals[verts*3] uvs[verts*2] joints[verts*4 ushorts] weights[verts*4] indices[indices ushorts]
Skin:      joint_nodes[joints ints] inverse_bind_matrices[joints*16 floats]
MeshNode:  mesh_node_global[16 floats]  (skinned mesh node's world transform)
DefaultTRS: default_t[num_nodes*3] default_r[num_nodes*4] default_s[num_nodes*3]  (bind-pose local TRS for ALL nodes)
Hierarchy: num_nodes(4) node_parents[num_nodes ints]
Anims:     for each anim { name(64 bytes) num_channels(4) for each ch { target_node(4) target_path(4) num_kf(4) times[kf] values[kf*3or4] } }
```

Key design decisions:
- **Default TRS**: Stored for every glTF node so that nodes without animation channels use their bind-pose values instead of identity. This is critical — 14+ joint nodes (Root, Hips, Shoulders, etc.) have no animation channels but non-trivial bind-pose rotations/translations.
- **No IBM pre-multiplication**: `joint_global * IBM` at bind pose already equals `mesh_node_global`. Pre-multiplying would double the 100x scale.
- **JOINTS_0 are skin-local indices** (0..31), not glTF node IDs — passed through as-is.

### Conversion Output

| Character | .skl Size | Verts | Indices | Joints | Nodes | Animations |
|-----------|----------|-------|---------|--------|-------|------------|
| Cleric | 263 KB | 1387 | 3444 | 32 | 50 | 15 |
| Monk | 277 KB | 2578 | 4902 | 32 | 48 | 11 |
| Ranger | 302 KB | 2336 | 4851 | 32 | 52 | 14 |
| **Rogue** | **203 KB** | **1037** | **3174** | **32** | **54** | **12** |
| Warrior | 263 KB | 1560 | 4536 | 32 | 51 | 14 |
| Wizard | 252 KB | 1182 | 3726 | 32 | 53 | 15 |

### Animation Clips (per character)

All 6 characters share the same animation rig:
- `CharacterArmature|Idle` — 9 channels
- `CharacterArmature|Walk` — 28 channels
- `CharacterArmature|Run` — 16 channels
- `CharacterArmature|Attacking_Idle` — 21 channels
- `CharacterArmature|Death` — 21 channels
- `CharacterArmature|PickUp` — 18 channels
- `CharacterArmature|Punch` — 29 channels
- `CharacterArmature|Dagger_Attack` / `Sword_Attack` / etc. — weapon-specific
- `CharacterArmature|Roll` — 24 channels
- `CharacterArmature|RecieveHit` — 16 channels

### File Layout (after conversion)

```
./assets/models/characters/
├── Cleric_out/
│   ├── Cleric.gltf
│   └── buffer.bin
├── Monk_out/
│   ├── Monk.gltf
│   └── buffer.bin
├── Ranger_out/
│   ├── Ranger.gltf
│   └── buffer.bin
├── Rogue_out/
│   ├── Rogue.gltf          ← Default player character source
│   └── buffer.bin
├── Warrior_out/
│   ├── Warrior.gltf
│   └── buffer.bin
├── Wizard_out/
│   ├── Wizard.gltf
│   └── buffer.bin
├── Cleric.skl              ← Compiled skeletal binary
├── Monk.skl
├── Ranger.skl
├── Rogue.skl               ← Default player character
├── Warrior.skl
├── Wizard.skl
├── Cleric_Texture.png
├── Monk_Texture.png
├── Ranger_Texture.png
├── Rogue_Texture.png        ← Default player texture
├── Warrior_Texture.png
├── Wizard_Texture.png
└── License.txt
```

### Project Asset Footprint

The full Quaternius packs are large (~152 MB, 878 files). The demo only uses a small subset, so the project keeps only the actively used files and archives the full packs externally:

- **Project (`./assets/models/quaternius/`):** ~65 files, ~7 MB
  - 5 tree variants + 6 textures
  - 8 building variants + 8 palette textures
  - 6 nature variants + 3 textures
  - 5 Medieval Village props (Well, Bonfire, MarketStand_1, Cart, Cauldron)
- **Archive (`~/assets/quaternius/`):** 878+ files, ~152 MB
  - Complete original Quaternius packs (trees, buildings, nature, medieval village) for future expansion

### Active Building Variants (Ultimate Buildings Pack — Dec 2019)

The 8 loaded building variants and their runtime doorstep configuration (used by `get_surface_height()` so the player’s feet align when entering a building):

| Variant | Palette Texture | Doorstep? | `step_raw` |
|---------|-----------------|-----------|------------|
| 1Story | `Texture_Light.png` | No | 0.00 |
| 2Story_GableRoof | `Texture_Light2.png` | No | 0.00 |
| 2Story_Balcony | `Texture_Yellow.png` | Yes | 0.13 |
| 2Story_Columns | `Texture_Grey.png` | No | 0.00 |
| 3Story_Small | `Texture_Dark.png` | No | 0.00 |
| 3Story_Balcony | `Texture_Red.png` | Yes | 0.13 |
| 4Story_Center | `Texture_Blue.png` | Yes | 0.13 |
| 4Story | `Texture_Green.png` | Yes | 0.13 |

Values were determined by an in-game survey: walk up to a building, press `F11` to record the position, then `F9` (no step) or `F10` (has step) to mark the nearest variant. The script prints the final table on exit.

## Runtime Loading

### glTF Node Hierarchy (Rogue example, 54 nodes)

```
0: RootNode [children: 1, 53]
├── 1: CharacterArmature [scale=100, rot=-90°X]  ← skeleton root
│   └── 2: Root [rot=+90°X]  ← first joint, no animation channels
│       ├── 3: Foot.L → 4: Shoelace.L (mesh) → 5: Foot.L_end
│       ├── 6: Body → 7: Hips → 8: Abdomen → ... → 11: Torso
│       │   ├── 12: Neck → 13: Head → 14: Face (mesh) → 15: Head_end
│       │   ├── 16: Shoulder.L → ... → 18: LowerArm.L → ...
│       │   └── 27: Shoulder.R → ... → 29: LowerArm.R → ...
│       ├── 46: PoleTarget.L → 47: end
│       ├── 48: Foot.R → 49: Shoelace.R (mesh) → 50: end
│       └── 51: PoleTarget.R → 52: end
└── 53: Rogue [scale=100, rot=-90°X, trans=[0,0,0.166], mesh=7, skin=0]  ← skinned mesh
```

Key observations:
- Nodes 1 and 53 share identical transforms (scale=100, rot=-90°X) — this is the FBX2glTF convention.
- `joint_global * IBM` at bind pose already equals `mesh_node_global` — no pre-multiplication needed.
- 14 joint nodes have **no animation channels** and must use bind-pose default TRS.

### Binary Buffer Loading
The `.bin` file is loaded via `fopen`/`fread` into `c_alloc`'d memory, then unpacked using `c_get_float`, `c_get_int16`, etc. based on accessor component types.

### GPU Upload
VBOs created for:
- Position (VEC3, FLOAT)
- Normal (VEC3, FLOAT)
- UV (VEC2, FLOAT)
- Joint indices (VEC4, UNSIGNED_SHORT — componentType 5123)
- Weights (VEC4, FLOAT)
- Index buffer (SCALAR, UNSIGNED_SHORT — componentType 5123)

### Animation System
- **States:** Idle (not moving), Walk (moving), Run (sprinting with Shift)
- **Looping:** `skel_get_anim_duration()` returns actual animation length; `player_anim_time % duration` wraps correctly
- **Keyframe sampling:** Linear search through `input` times, lerp translation/scale, slerp rotation
- **Hierarchy traversal:** Compute world matrices from local transforms for ALL 54 nodes, multiply by inverse bind matrix for 32 joints, upload as `uniform mat4 u_jointMatrices[32]`
- **Default TRS:** Bind-pose values loaded from .skl for all nodes; animation channels override only targeted nodes

### Shaders
Vertex shader (`#version 330`):
```glsl
uniform mat4 u_mvp;
uniform mat4 u_jointMatrices[32];
in vec3 a_position;
in vec2 a_uv;
in vec4 a_joints;
in vec4 a_weights;
out vec2 v_uv;
void main() {
  mat4 skinMatrix = a_weights.x * u_jointMatrices[int(a_joints.x)]
                  + a_weights.y * u_jointMatrices[int(a_joints.y)]
                  + a_weights.z * u_jointMatrices[int(a_joints.z)]
                  + a_weights.w * u_jointMatrices[int(a_joints.w)];
  gl_Position = u_mvp * skinMatrix * vec4(a_position, 1.0);
  v_uv = a_uv;
}
```

Fragment shader:
```glsl
uniform sampler2D u_texture;
in vec2 v_uv;
out vec4 fragColor;
void main() {
  fragColor = texture(u_texture, v_uv);
}
```

### Player Rendering
- `draw_player_skeletal()` applies translate → rotate (yaw) → rotate (180° Y to face away from camera) → scale (0.25)
- Reads combined MVP from OpenGL matrix stack, passes to `skel_draw()`
- `player_pos[1] = 0.05` to prevent feet clipping through floor
- Each character has its own texture loaded at startup; Tab cycles through all 6

## Scene Polygon Budget

Actual startup-reported static triangle count: **~207,984** (buildings + trees + grass + rocks + props + roads + curbs + water + markers + ground plane). The player skeletal model adds ~3,000–5,000 dynamic tris on top.

Approximate breakdown (with default generation):

| Object | Count | Avg tris | Total tris |
|--------|-------|----------|------------|
| Buildings | ~40 | ~4,400 | ~176,000 |
| Trees | ~34 | ~1,100 | ~37,400 |
| Grass clumps | ~85 | ~260 | ~22,100 |
| Rocks | ~68 | ~70 | ~4,760 |
| Props | 5 | varies | ~1,500 |
| Roads (merged) | 5 segments | 12 | ~60 |
| Road curbs | ~30 segments | 12 | ~360 |
| Water/ponds/river/moat | ~60 instances | 12 | ~720 |
| Grass floor | 1 quad | 2 | ~2 |
| Player (skeletal) | 1 | varies | ~3,000–5,000 |
| **Static total** | — | — | **~207,984** |
| **With player** | — | — | **~211,000–213,000** |

The bottleneck on this demo is not raw triangle count but **draw calls and interpreter overhead**. Profile shows `draw_scene` at ~10.5 s per 100 frames, with the heaviest functions being textured model groups (~3.2 s), water VBO (~3.8 s before conversion, now reduced), sky (~2.2 s before VBO conversion), and particles (~1.3 s before VBO conversion).

## Grass Floor Texture

- Source: `grass_55.png` (OpenGameArt, CC0)
- Resolution: 512×512, seamless tiling
- Loaded via existing `load_png_texture()` function
- Applied to 80×80 floor with 16×16 UV repeat for dense grass appearance

## License Notes

All models and textures are **CC0** (public domain equivalent). No attribution required. The FBX2glTF converter is BSD-3-Clause licensed (Facebook/Meta open source).

## Reproducing This Pipeline

1. Download the RPG pack: `curl -L -o rpg_characters.zip "https://opengameart.org/sites/default/files/rpg_characters_-_nov_2020.zip"`
2. Download grass texture: `curl -L -o grass_55.png "https://opengameart.org/sites/default/files/grass_55.png"`
3. Install converter: `npm install fbx2gltf`
4. Convert FBX files to glTF (see Stage 1 commands above)
5. Copy glTF outputs to `./assets/models/characters/`
6. Run `python3 ./process_gltf.py ./assets/models/characters/*_out/*.gltf` to generate .skl files
7. Compile libskel.so: `gcc -shared -fPIC -O2 -o ./libskel.so /tmp/libskel.c -lGL -lm`
8. Copy grass to `./assets/textures/`

Last updated: 2026-07-26 (Medieval Village props added; polygon budget updated to actual startup count ~207,984; Kenney player model removed)
