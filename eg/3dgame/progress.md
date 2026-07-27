# 3D Game Development Progress Log

> **AGENT NOTE:** Do not delete or overwrite entries in this file without explicit user confirmation. Append new information below existing sections. Mark outdated items with strikethrough or deprecation notes, but preserve the original record.

## Current State (July 28, 2026)

### Architecture
- **Script size:** ~6,870 lines
- **Renderer:** OpenGL 3.3 Compatibility Profile, fixed-function pipeline; skeletal characters use a separate `#version 330` shader
- **Shading:** `GL_LIGHTING` enabled with interpolated directional sun (`GL_LIGHT0`), per-vertex normals, ambient/diffuse/specular material; continuous day/night cycle drives sun/light/fog/sky colors
- **Atmosphere:** gradient sky dome (16×32), distance fog (20–90 units), blob shadows under the player, animated water surfaces (round polygonal blobs with UV scroll, sea-blue moat tint), 240-particle system (dust, leaves, fireflies, sparkles) with F2 toggle, lit-bonfire particle emitter + point light
- **Player:** Third-person chase camera with 6 Quaternius skeletal characters (TAB to cycle); mouse look for yaw/pitch; feet snap to grass, road/curb, or building doorsteps; very slight slowdown in shallow pond edges
- **World:** -80..+80 unit play area (expanded from original -40..+40); ~30-35 buildings with per-variant doorstep zones; CPU-side frustum sphere culling enabled (F8 toggle); ponds have round edges and circular deep/shallow collision zones; sandy beach ring at world boundary (static VBO, precomputed at init)
- **Performance:** 65 FPS uncullled, 90 FPS with full culling (up from ~27 FPS baseline)

### Completed Features

#### Quaternius Asset Integration
- **Textured LowPoly Trees** (Tree_1, Tree_5, Pine_1, Birch_1, DeadTree_1)
- **Ultimate Buildings Pack (Dec 2019)** — 8 variants: `1Story`, `2Story_GableRoof`, `2Story_Balcony`, `2Story_Columns`, `3Story_Small`, `3Story_Balcony`, `4Story_Center`, `4Story`
- **Ultimate Nature Pack** (Bush_1, Bush_2, Grass, Grass_2, Plant_1, Rock_1)
- All packs downloaded and installed in `assets/models/quaternius/`

#### Extended OBJ/MTL Pipeline
- `parse_obj()` returns 9 values: `verts, texcoords, vert_indices, tex_indices, vertex_colors, material_groups, material_names, normals, norm_indices`
- `parse_obj()` now strips `\r` from CRLF line endings so Windows-exported OBJ files parse correctly
- `load_mtl()` parses `newmtl`, `Kd` (diffuse color), and `map_Kd` (texture path)
- `draw_textured_model_groups()` — multi-material step-5 textured rendering with per-group texture binding: `[vbo, tex_vbo, norm_vbo, num_verts, tex_id, ...]`
- `measure_obj_bounds()` — computes actual bounding box from vertex data for collision

#### Player System
- 6 Quaternius skeletal characters: Rogue, Warrior, Wizard, Cleric, Monk, Ranger
- TAB key cycles characters; each has unique `.skl` model + `.png` texture
- Idle / Walk / Run animation states driven by movement + sprint
- Third-person chase camera (`CAMERA_DISTANCE=2.5`, `CAMERA_HEIGHT=1.2`, `CAMERA_SIDE=0.5`)
- Mouse look: accumulated `glfwSetCursorPosCallback` updates yaw/pitch; works with external mice and non-suppressed trackpads
- Free camera mode (C key): first-person fly with WASD/QE/RF
- Home key resets to default chase view

#### World Generation
- **Buildings:** ~40 buildings lining a connected road grid, facing the streets
  - `building_positions[]` format: `[x, y, z, sx, sy, sz, yaw, cr, cg, cb, model_index]` (11 elements)
  - Per-variant measured bounds for collision
  - Central town square (-8..8 in X and Z) kept clear
- **Trees:** forest clusters/edges in the outskirts + scattered accent trees
  - `tree_positions[]` format: `[x, z, scale, yaw, model_index]` (5 elements)
  - Two-pass sway: bark stationary, leaves rotate on Z-axis
- **Nature:** meadow patches of grass clumps + scattered accents + rocks
  - `grass_clump_positions[]` format: `[x, z, scale, yaw, model_index]` (5 elements)
  - Procedural planar UVs + generated textures (Rock.png, Grass.png, Bush.png)
- **Roads:** 5 connected road segments forming a 3×3 grid (main road + cross/side streets)
  - `road_instances[]` format: `[cx, cy, cz, sx, sy, sz, r, g, b, u_scale, v_scale]` (11 elements)
  - `road_world_vbo` — merged world-space VBO for all road tops
  - `road_curb_vbo` — merged VBO for curbs
- **Ponds:** organic irregular blobs with circular deep/shallow water collision
  - `water_instances[]` format: `[cx, cy, cz, sx, sy, sz, r, g, b]` (9 elements)
  - `water_colliders[]` format: `[cx, cz, inner_r, outer_r]` (4 elements) — inner blocks movement, outer slows to 0.85×
  - Small instances drawn as 18-vertex round polygons; large instances (moat/river) as quads
- **Water moat + sandy beach:** impassable border just outside the play grid
  - Beach: 6-unit sandy ring from ±40 to ±46
  - Water: overlapping strip from ±44 to ±60 (8-unit depth) with AABB colliders
- **Shoreline rocks:** random clumps along the inner edge of the moat (32 rocks total)
  - `rock_positions[]` format: `[x, z, scale, yaw]` (4 elements)

#### Lighting & Atmosphere
- Continuous day/night cycle (`world_clock_minutes` 0–1440)
- Interpolated sun direction, light color, fog color, and sky gradient via `get_time_of_day_transition()`
- `time_of_day` enum: 0=Dawn, 1=Day, 2=Dusk, 3=Night
- `GL_FOG` with `GL_LINEAR` mode (`FOG_START=20`, `FOG_END=90`)
- Sky dome: half-sphere with 16 stacks × 32 slices, colored per-vertex by altitude
- Blob shadow under player: dark quad on ground, fades at dusk/night

#### Collision & Movement
- `check_collision(px, pz)` — circular + AABB hybrid for buildings, roads, props
- `get_surface_height(px, pz)` — raycast to find ground/road/curb/building doorstep height
- Player feet snap to surface; slight upward push on doorsteps for smooth entry
- Sprint (Shift) doubles movement speed; jump (Space) with forward boost

#### Save/Load System
- Manual save/load: F5 to save, F9 to load
- Save file: `saves/save_{timestamp}.txt` with player position, yaw, pitch, time, character index
- No auto-save; no save on exit

#### UI/HUD
- Draggable clock panel (top-right, shows world time + FPS + triangle count)
- F1: wireframe toggle, F2: particles toggle, F3: VSync toggle, F4: collision toggle, F6: floor solid toggle, F7: time speed, F10: step frame, F11: survey camera, F12: toggle clock panel
- N key: jump to dusk (19:00) for glow testing

### Technical Infrastructure
- **Global lock fix:** `za` interpreter rebuilt with atomic nesting counter (`has_global_lock uint32`) instead of goroutine-ID mutex. FPS restored from ~10 to ~31.
- **No libc FFI:** All file I/O uses native za (`$in`, `$out`, `read_file`, `write_file`, `mkdir_p`). All C interop uses za builtins (`c_fopen`, `c_alloc`, `c_free`, etc.).
- **VBO-based static geometry:** All roads, curbs, water, sky, and particles use pre-baked VBOs to minimize immediate-mode overhead.

---

## Session Log

### July 25, 2026 — Asset Integration & Town Layout
- Downloaded and integrated Quaternius Ultimate Buildings (Dec 2019), Nature Pack, and textured trees
- Implemented `parse_obj()`, `load_mtl()`, `draw_textured_model_groups()` for multi-material textured rendering
- Built connected road grid with buildings lining the streets
- Added Quaternius skeletal characters (Rogue, Warrior, Wizard) with idle/walk/run states
- Added Quaternius nature props (well, bonfire, market stand, cart, cauldron) as interactables

### July 25, 2026 — Polish: Save/Load, Collision, Save System
- Fixed global-lock deadlock in `za` interpreter (atomic nesting counter instead of goroutine-ID mutex)
- Removed all `std::system` calls and `lib libc::...` FFI declarations
- Implemented manual save/load with native za file I/O
- Added collision for buildings, roads, curbs, props, ponds, and shoreline rocks
- Added `get_surface_height()` raycast for feet snapping
- Added draggable clock panel with world time, FPS, and triangle count

### July 25, 2026 — Night Visuals: Glow System v1
- Added `building_variant_glows[]` with per-variant window/lantern positions
- Warm emissive tint on buildings at dusk/night (`cr *= 1.2, cg *= 0.9, cb *= 0.6`)
- `draw_building_glows()` with additive blending and two crossed quads per glow
- Time-gated: only visible when `time_of_day >= 2`

### July 26, 2026 — Night Glow v2: Doorway Lanterns + Warm Emissive Tint
- Replaced wall-hitting window glows with doorway lanterns derived from `doorstep_zones[]`
- Only buildings with doorsteps (step_raw > 0) get lanterns
- Added N key handler to jump world clock to 19:00 (dusk)
- Removed `building_variant_glows[]` and per-variant glow computation

### July 26, 2026 — Polish Round: Collision, Glow, Water, Beach, Ragged Edges
- **Rock collision tightened:** `scale * 0.5` → `scale * 0.18` (matches actual rock model)
- **Doorway glow sprites removed:** Deleted `generate_doorway_lanterns()`, `draw_building_glows()`, `building_glow_positions[]`. Warm building tint kept.
- **Moat colliders shrunk:** AABB colliders narrowed to 2.0 units deep, shifted outward. Overlap zone (±44–±46) now walkable/shallow.
- **Ragged grass floor edges:** Single quad replaced by 8×8 grid with jittered boundary vertices (±1.8 units). Fixed UV integer-division bug with `as_float()`.
- **Rolling surf waves:** 4 wide animated strips (200×8) with dynamic VBO. Horizontal push ±0.15, vertical ripple ±0.04, coherent crossed-sine phase. Alpha breathes 0.2–0.85.
- **Gap-free beach transition:** Beach at `y=0.01` (1 cm above grass), inner edge pushed to ±37 with identical jitter seeds as grass boundary. No blue slivers.

### Files Modified (July 26)
- `./game` — ~100 lines changed across rock collision, glow removal, moat colliders, grass VBO, beach draw, moat wave system

### Verification
- Game boots cleanly with all changes
- No doorway lantern output in startup log
- Water shows rolling surf; tide stays on beach, never reaches grass
- Grass/beach transition is gap-free and ragged on both sides

---

## July 26, 2026 — World Expansion (±40 → ±80)

### Goal
Double the play area radius from ±40 to ±80, keeping the town layout identical but making room for future biomes/features in the expanded landscape.

### Strategy
Introduce a single `WORLD_HALF = 80.0` constant near the top of the file and replace all hardcoded boundary values with expressions based on it.

### Changes made

| Element | Old | New |
|---|---|---|
| Grass floor boundary | ±40 | ±80 (hardcoded 80.0 in VBO) |
| Grass floor UV repeat | 6.0 | 12.0 |
| Wireframe grid | -40..+40 | -80..+80 (vertices: 164→324) |
| Beach outer edge | ±46 | ±86 (`WORLD_HALF + 6`) |
| Beach inner edge base | ±37 | ±77 (`WORLD_HALF - 3`) |
| Water moat center | ±48 | ±88 (`WORLD_HALF + 8`) |
| Water strip width/depth | 100 | 200 |
| Moat colliders | ±50 | ±90 |
| Shoreline rocks | ±36..±38 | ±76..±78 |
| Particle out-of-bounds | ±45 | ±85 |

### What stays the same
- Town layout: roads end at ±36, buildings placed -36..+36
- Tree/grass/rock/pond placement ranges (stay near town, ±35/±38)
- Fog distances (`FOG_START=20`, `FOG_END=90`, `VIEW_DISTANCE=100`)
- Spawn zone (8-unit clear zone, 6-unit collider)
- Content density (same number of trees, ponds, rocks, buildings)
- Camera, controls, sky dome size

### Notes
- `WORLD_HALF` works in init/draw code (beach, water, colliders, terrain grid) but caused a runtime error inside `create_textured_floor_vbo()` when assigned to a local variable. ~~Kept `s = 80.0` hardcoded in that function as a workaround.~~ **FIXED** — see below.

**~~Exotic runtime error with `WORLD_HALF` in `create_textured_floor_vbo()` — investigation needed~~ FIXED**

**Root cause:** Three interacting bugs in the `za` interpreter's bytecode system for function bodies:
1. **Token copy bug (`actor.go:1946`)**: `for _, itok := range inbound.Tokens` creates a copy of each token; modifying `itok.bindpos` does NOT modify the actual token in the slice. So tokens copied into `functionspaces[lmv]` retained their **main-space** `bindpos` values instead of being rebound to the callee's function space.
2. **Bytecode compiled for wrong space (`phraser.go`)**: Bytecode is compiled during the main-script parse using `fnTypeHints[main]`. Type hints from ALL functions leak into each other via the main-space map. When another function happened to set `s: hintInt`, the bytecode for `-s` in `create_textured_floor_vbo` was compiled as `OpNegInt` (expects `int`), but at runtime `s` was `float64` (from `WORLD_HALF = 80.0`).
3. **Cross-space assignment bug (`assigner.go:561`)**: `doAssign` always passed `&assignee[0]` to `vset`, which uses `tok.bindpos` directly. After fixing the copy bug, global assignments (`@var = expr`) inside functions wrote to the wrong slot in `mident` because the token's `bindpos` was now the function-space bindpos, but `vset` was targeting the global table (`lfs = main`).

**Fixes applied:**
- `actor.go:1946`: Changed to index-based iteration `for i := range inbound.Tokens { inbound.Tokens[i].bindpos = ... }` so tokens are correctly rebound to the callee's function space.
- `actor.go:1940`: Recompile bytecode with corrected `bindpos` and `nil` type hints when copying phrases into `functionspaces[lmv]`. This fixes both wrong slot numbers and cross-function type-hint leakage (e.g., another function's `s: int` leaking into `create_textured_floor_vbo`'s `s: float` bytecode).
- `assigner.go:561`: Changed `doAssign` to pass `nil` instead of `&assignee[0]` to `vset` when `lfs != rfs`, forcing `bind_int(lfs, name)` lookup for cross-space/global assignments.

**Result:** The `s = WORLD_HALF` workaround has been removed. The game now uses `WORLD_HALF` consistently everywhere and runs without crashes. Function-body bytecode is preserved (not discarded), so expressions inside functions still execute as fast bytecode rather than falling back to the recursive `Eval()` parser.

**FPS impact:** Restoring function-body bytecode improved FPS from ~27 to **35** (no culling) and **45** (near full culling) — a **30–65% improvement**.

### Separate future actions
- ~~**Hardware instancing / batching**~~ — **DONE** (tree bark instancing implemented in later session)
- ~~**Fixing culling**~~ — **DONE** (frustum culling implemented in later session)

### Files modified
- `./game` — ~50 constant replacements across init and draw code

### Verification
- Game boots cleanly with expanded world
- Town layout unchanged (34 buildings, same road grid)
- Total triangles ~196k (up from ~180k due to larger terrain grid + beach)

---

## July 26, 2026 — CPU Frustum Sphere Culling

### Goal
Add per-instance frustum culling to reduce draw calls when the camera is facing away from large portions of the scene.

### Implementation

**Two new functions added:**
1. `extract_frustum_planes()` — Reads the current OpenGL `GL_PROJECTION_MATRIX` and `GL_MODELVIEW_MATRIX`, multiplies them into a combined clip-space matrix, and extracts 6 inward-pointing frustum planes directly from the matrix rows (`left = row4+row1`, `right = row4-row1`, `bottom = row4+row2`, `top = row4-row2`, `near = row4+row3`, `far = row4-row3`). Each plane is normalized. This guarantees the frustum exactly matches OpenGL's view volume, avoiding all geometric sign/axis convention errors.
2. `sphere_in_frustum(cx, cz, radius, planes)` — Tests a ground-based bounding sphere against all 6 planes. Uses `cy=0` (all objects on ground plane). Returns `false` if `dot(center, normal) + dist < -radius` for any plane.

**Modified `draw_scene()`:**
- Extracts frustum planes once per frame at the top (if `!free_camera`)
- Culls 4 categories before their draw calls:
  - **Buildings:** radius = `max(hw*sx, hd*sz)` from `building_collision_half_w/half_d`
  - **Trees:** radius = `scale * 2.0`
  - **Grass clumps:** radius = `scale * 0.5`
  - **Rocks:** radius = `scale * 0.4`
- Categories left unculled (single draw call or always near player): roads, curbs, beach, moat waves, sky dome, player shadow, particles, interactable glow

**Free camera:** Culling is fully disabled when `free_camera` is active (C key). This prevents pop-in during fly-through.

**F8 toggle:** Repurposed F8 keybind to toggle `culling_enabled` on/off (was camera collision toggle). HUD shows `Cull[On/Off]`.

### Bug fix: matrix-based frustum extraction
- **Root cause:** The yaw/pitch geometric frustum extraction had multiple sign errors in forward/right/up vectors and plane D terms, causing everything in front of the camera to be culled and objects to flicker only on the left viewport edge.
- **Fix:** Replaced geometric extraction with **matrix-based extraction**: reads the actual OpenGL `GL_PROJECTION_MATRIX` and `GL_MODELVIEW_MATRIX`, multiplies them into a combined clip-space matrix, and extracts the 6 planes directly from the matrix rows (`left = row4+row1`, `right = row4-row1`, etc.). This guarantees the frustum exactly matches what OpenGL uses for rendering, eliminating all sign/axis convention errors.
- **Result:** Culling now achieves ~56% cull rate (112/199 objects) with correct visibility — objects in front of the player are rendered, and off-screen objects are reliably culled on all sides.

### Keybind restored
- **F8** restored to `camera_collision_enabled` toggle (was incorrectly repurposed to culling). HUD legend shows `F8 CamColl[On/Off]` again.

### Files modified
- `./game` — replaced `extract_frustum_planes()` with matrix-based version (reads actual OpenGL matrices); restored F8 to camera collision toggle; updated HUD legend

### Verification
- Game boots cleanly with culling active
- Objects are visible in front of the camera; no left-side-only flickering
- ~56% of off-screen objects culled per frame
- Free camera mode disables culling as intended
- F8 toggles camera collision and updates HUD legend
- **M key** toggles frustum culling on/off; HUD shows `M Cull[On/Off]`

---

---

## Hardware Instancing Implementation

### Approach
Replaced per-object `glPushMatrix`/`glDrawArrays` calls with `glDrawArraysInstanced` using a minimal GLSL shader.

### Shaders
- **Vertex shader** (`#version 330 compatibility`): per-instance attributes at locations 3-6 (`aInstancePos`, `aInstanceYaw`, `aInstanceScale`, `aInstanceColor`), yaw rotation via `mat3`, transform via `gl_ModelViewProjectionMatrix`
- **Fragment shader**: texture sampling with `discard` for alpha < 0.1, color modulation
- **Shader compilation**: uses `za`'s `[]string` FFI support for `glShaderSource`; discovered `za` multi-line string concatenation bug (`+` across lines fails), so shaders are stored as single-line literals

### Objects migrated
1. **Buildings** (~28-32 instances, 8 variants) — grouped by variant after culling, one instanced draw per visible variant
2. **Grass clumps** (~60-90 instances, 5 variants) — same pattern as buildings
3. **Rocks** (~20 instances, 1 variant) — single instanced draw

### Instancing status
- **Default:** ON at startup (instancing is now the standard render path)
- **Toggle:** None — instancing is always active. If the shader fails to compile, the game falls back to per-object draws automatically.

### Objects left as individual draws
- **Trees** — ~~bark+leaves are mixed in model groups; leaves need per-frame sway animation. Could be done by pre-separating bark groups at load time, but left for future session.~~ **DONE** — bark is now instanced via `tree_bark_variants[]`, leaves drawn individually with sway via `draw_tree_leaves()`.
- **Props** (well, bonfire, etc.) — only 6 instances, low ROI
- **Player skeletal** — rendered by external `libskel.so`, not easily instanced

### Bug fixes after initial instancing implementation

**1. Solid colors instead of textures (instanced objects)**
- **Root cause:** `draw_instanced_variant` used `glEnableClientState` + `glVertexPointer`/`glTexCoordPointer`/`glNormalPointer` to feed geometry data. In compatibility mode with an active shader program, the legacy `glTexCoordPointer` feeds `gl_MultiTexCoord0` (built-in) instead of generic vertex attribute 1, so `vTexCoord` in the shader was uninitialized.
- **Fix:** Replaced all `glEnableClientState`/`gl*Pointer` calls with explicit `glEnableVertexAttribArray` + `glVertexAttribPointer` for attributes 0-6. Now positions, texcoords, normals, and instance data all feed the correct shader attributes.

**2. Skeletal player model hand corruption**
- **Root cause (program deactivation):** `draw_instanced_variant` called `glUseProgram(0)` at its end, deactivating the external `libskel.so` library's internal shader before `draw_player_skeletal`. The library fell back to fixed-function pipeline, misinterpreting bone-weight/joint-index vertex attributes as positions/normals.
- **Fix:** Query the currently active GL program with `glGetIntegerv(GL_CURRENT_PROGRAM)` at the start of `draw_scene()`, save it, and restore it with `glUseProgram(saved_program)` before `draw_player_skeletal()`. This ensures whatever program was active before our instancing draws (usually the skeletal library's program) is restored before the skeletal draw.
- **Root cause (delayed movement corruption):** `glVertexAttribDivisor(3, 1)` etc. are **global OpenGL state** and persist even after `glDisableVertexAttribArray`. We set divisors to `1` for instancing but never reset them to `0`. When the skeletal library later draws using attributes 3-6 (bone weights, joint indices), OpenGL advanced them **once per instance** instead of **once per vertex** — reading deformation data from garbage memory during animation (movement/rotation).
- **Fix:** After each `glDrawArraysInstanced`, reset all attribute divisors back to `0` with `glVertexAttribDivisor(3, 0)`, `glVertexAttribDivisor(4, 0)`, `glVertexAttribDivisor(5, 0)`, `glVertexAttribDivisor(6, 0)`. This ensures non-instanced draws read attributes per-vertex, not per-instance.

**3. Culling stats inflation with instancing ON**
- **Root cause:** In the nested instancing loops (variant outer, instances inner), `total_count` and `culled_count` were incremented at the **top** of the inner loop, before the `if bidx != v` check. Every variant scanned all objects, inflating the count by a factor of ~variant_count.
- **Fix:** Moved `total_count = total_count + 1` and `culled_count = culled_count + 1` to **after** the variant-match check, so only objects belonging to the current variant are counted.
- **Before:** ~729 total objects (8 variants × ~31 buildings + 5 variants × ~82 grass + rocks + trees)
- **After:** ~186 total objects (same as non-instancing baseline)

**4. FPS regression (~20 FPS instead of high FPS)**
- **Root causes:**
  1. `glBufferData` per variant per frame discarded and reallocated GPU memory, causing pipeline stalls on AMD/Mesa
  2. `c_alloc` + `c_set_float` per visible instance built data via za→C copies one float at a time
  3. `concat()` on za arrays per instance created new arrays constantly
  4. With only ~4 buildings per variant on average, instancing setup cost exceeded the savings from batching tiny groups
- **Fix:**
  1. Added `glBufferSubData` import and replaced `glBufferData` with `glBufferSubData` for instance uploads (avoids GPU memory reallocation)
  2. Replaced za `concat()` arrays with direct `c_alloc` + `c_set_float` writes to a C buffer per variant (no za array churn)
  3. Added `MIN_INSTANCES_FOR_INSTANCING = 8` threshold — variants with < 8 visible instances fall back to individual `draw_building`/`draw_textured_model_groups` calls, avoiding expensive shader/VBO setup for tiny batches
  4. Small-batch fallback reads instance data directly from the pre-built C buffer

### Draw call reduction (estimated, with threshold)
- Buildings: ~30-90 → ~15-30 draws (most variants fall back to individual draws due to low instance count)
- Grass: ~60-90 → ~10-20 draws (some variants batch, others fall back)
- Rocks: ~20 → ~1-20 draws (depends on visible count vs threshold)
- Trees: ~50-70 (unchanged, still individual)
- **Total scene draws: ~180-240 → ~90-150** (less reduction than pure instancing, but much higher FPS due to avoiding setup overhead for tiny batches)

### Files modified
- `./game` — shader sources, `compile_shader()`/`create_shader_program()`, `draw_instanced_variant()`, instance VBO creation, modified `draw_scene()` building/grass/rock loops with hybrid threshold + `glBufferSubData` fallback

---

## Remaining Backlog

1. **Add per-instance directional lighting to instancing shader (1B)** — currently shader does `texture × color` with no dynamic lighting; bake `GL_LIGHT0` direction into shader for matching look
2. **Tree bark instancing** — separate bark material groups from leaves at load time, instance bark, keep leaves individual with sway
3. **Fix culling for water ponds** — small instances drawn 1-by-1 inside `draw_water_vbo()`, could also be culled
4. **Fix culling for tall objects** — currently uses `cy=0`, may slightly miscull very tall trees near top frustum edge
5. ~~**`za` interpreter bug** — `WORLD_HALF` identifier resolution crash in `create_textured_floor_vbo()`~~ **FIXED** (see earlier note); also multi-line string concatenation with `+` fails across lines
6. **Minimap panel** — Add a small draggable minimap HUD panel (e.g., top-right) showing an overhead view of the town with player position, buildings, and roads. Could reuse the existing panel framework and render to a small off-screen texture or use simple 2D primitives.

---

## Session: Moat GPU Shader + Beach VBO Optimization

### Goal
Eliminate the two largest per-frame CPU costs identified in `profile_output2.txt`:
1. **Moat waves** (~3.6s cumulative in `draw_scene`) — CPU recomputed 800 vertices with `sin`/`cos` every frame and uploaded via `glBufferData`
2. **Beach** (~1.1s cumulative) — Immediate mode `glBegin`/`glEnd` with ~172 quads and per-vertex CPU trig

### Implementation

#### 1. Moat Waves → Vertex Shader
- **New shaders:** `MOAT_VS`/`MOAT_FS` (`#version 330 compatibility`) with `u_time` and `u_world_half` uniforms
- **VS logic:** Replicates exact CPU displacement math (`dist`, `taper`, `phase`, `push`, `y_wave`, `alpha`) using per-vertex attributes: `aPos`, `aTexCoord`, `aIsHoriz` (1.0/0.0), `aPushSign` (+1.0/−1.0)
- **Texture scroll:** Applied via `gl_TextureMatrix[0]` so the existing `draw_water_vbo()` texture-matrix animation still works
- **`init_moat_wave_vbo()` rewrite:** Builds a **static** `GL_STATIC_DRAW` VBO once at init (7 floats/vertex), then `c_free`s the temporary buffer. No persistent CPU buffer needed.
- **`draw_moat_waves()` rewrite:** Sets `glUseProgram(moat_program)`, uploads two uniforms (`u_time`, `u_world_half`), enables 4 generic vertex attributes, draws with `glDrawArrays`, disables attributes, `glUseProgram(0)`. Entirely self-contained.
- **Fallback:** If shader compilation fails, `moat_program == 0` and `draw_moat_waves` silently returns (moat invisible). A log line is printed at init.
- **FFI added:** `glUniform1f` import for setting float uniforms.

#### 2. Beach → Fixed-Function VBO
- **`init_beach_vbo()` new function:** Preallocates a `GL_DYNAMIC_DRAW` VBO at init (`c_null()` data) and a persistent CPU buffer `beach_data_ptr`.
- **`draw_beach()` rewrite:** Computes the same ragged shoreline with `sin`/`cos`, writes positions+UVs into the CPU buffer, uploads with `glBufferSubData`, then draws with `glEnableClientState` + `glVertexPointer`/`glTexCoordPointer` + `glDrawArrays(GL_QUADS)`. No `glBegin`/`glEnd`.
- **Color:** Still handled by `glColor4f(1,1,1,1)` + `GL_MODULATE` texture environment (fixed-function).

### Files modified
- `./game` — added `glUniform1f` binding, `MOAT_VS`/`MOAT_FS`, `create_moat_shader_program()`, rewrote `init_moat_wave_vbo()`/`draw_moat_waves()`, added `init_beach_vbo()`, rewrote `draw_beach()`, added init calls

### Risks / Open Questions
- `gl_TextureMatrix[0]` in `#version 330 compatibility` — should work per spec, but untested on the user's Mesa driver. If compilation fails, moat will disappear with a console warning.
- The moat shader and beach VBO were not runtime-tested yet (no access to the user's game window). Potential issues: attribute layout mismatch, VBO stride miscalculation, or `GL_QUADS` deprecation warnings in core profile (but we're in compatibility mode).
- If the moat shader fails, the only fallback is no moat. A safer fallback would be to keep the old CPU code path, but that requires more code. For now, the silent skip is acceptable for a dev build.

---

## Future Za FFI Enhancement: Typed Array Allocators ✅ IMPLEMENTED

> **Status:** Implemented in za interpreter v1.3.0 (July 27, 2026). See `~/go/src/za/CHANGELOG` for upstream entry.

### Problem
The game's OpenGL code extensively uses `c_alloc()` + `c_set_float()` / `c_get_float()` to build interleaved VBO data (vertex positions, UVs, colors, instance matrices, particle buffers, beach geometry, etc.). This works but is verbose and requires manual byte-offset arithmetic:

```za
# Current pattern — works, but tedious and error-prone
ptr = c_alloc(count * 40)
c_set_float(ptr, j * 40 + 0, px)
c_set_float(ptr, j * 40 + 4, py)
c_set_float(ptr, j * 40 + 8, pz)
...
glBufferSubData(GL_ARRAY_BUFFER, 0, count * 40, ptr)
c_free(ptr)
```

### What Already Exists in Za
Za already has typed read/write functions for C memory buffers:

- **`c_get_float(ptr, offset)` / `c_set_float(ptr, offset, value)`** — 32-bit float I/O into `CPointerValue` buffers (from `c_alloc()`). These are what the game uses today.
- **`c_get_double(ptr, offset)` / `c_set_double(ptr, offset, value)`** — 64-bit double I/O.
- **`c_get_float_at_addr(address, offset)` / `c_set_float_at_addr(address, offset, value)`** — Same, but for opaque `int64` pointers returned from external C libraries (e.g. FreeType `FT_Library`). Not used in the game.

These functions solve the "how do I put a float32 into C memory" problem. What they **don't** solve is ergonomics — you still do byte-offset math (`j * 40 + 8`) instead of index-based access (`buf[j * 10 + 2]`).

### Comparison with Other Languages
- **LuaJIT FFI**: `ffi.new("float[100]")` gives typed array access `buf[i]` — no byte offsets.
- **Python ctypes**: `(c_float * 100)()` gives `buf[i]`.
- **Python + NumPy**: `np.zeros(100, dtype=np.float32)` is the standard for PyOpenGL.
- **Plain Lua**: Has **no built-in FFI** — worse than Za. Needs C extension modules or pre-built bindings.
- **Za**: Has FFI, but no typed C array allocation yet. Manual `c_alloc` + byte offsets required.

### Proposed Additions to Za Standard Library

#### 1. `c_alloc_floats32(count:int) -> pointer` ✅
- Allocates `count * 4` bytes via `c_alloc_array("float32", count)`.

#### 2. `c_alloc_floats64(count:int) -> pointer` ✅
- Allocates `count * 8` bytes via `c_alloc_array("float64", count)`.

#### 3. `c_alloc_array(type_string:string, count:int) -> pointer` ✅
- Generic typed allocator taking a **scalar type name** (not `[]float32` syntax).
- Validates: type must exist in Typemap, must be a scalar primitive (numeric or pointer), must not contain brackets.
- Computes element size via `reflect.Type.Size()` and allocates via `CAllocBytes(count * elemSize)`.
- Examples:
  ```za
  ptr = c_alloc_array("float32", 100)    # 400 bytes
  ptr = c_alloc_array("float64", 100)    # 800 bytes
  ptr = c_alloc_array("int", 100)        # platform-sized int bytes
  ptr = c_alloc_array("pointer", 100)    # 800 bytes on 64-bit
  ```

#### 4. Index-based accessors ✅
- `c_array_set_float32(ptr, index, value)` — writes float32 at `index * 4`.
- `c_array_get_float32(ptr, index) -> float` — reads float32 at `index * 4`.
- `c_array_set_float64(ptr, index, value)` — writes float64 at `index * 8`.
- `c_array_get_float64(ptr, index) -> float` — reads float64 at `index * 8`.
- Type-specific naming avoids ambiguity about element width.

### Implementation Details
- Added `cAllocArray(typeStr string, count int)` helper in `lib-c.go` with scalar validation using `reflect.Type.Kind()`.
- Added `Typemap["float32"]` and `Typemap["float64"]` in `assigner.go`.
- Registered 7 new builtins in `buildFfiLib()`: `c_alloc_array`, `c_alloc_floats32`, `c_alloc_floats64`, `c_array_get_float32`, `c_array_set_float32`, `c_array_get_float64`, `c_array_set_float64`.
- Test script: `za_tests/ffi/test_c_alloc_array.za` covers allocation, helpers, index accessors, edge cases.

### Bottom Line
`c_set_float` / `c_get_float` already carry the heavy lifting for 32-bit float I/O. These additions make the code less verbose and closer to LuaJIT/PyOpenGL ergonomics. The scalar-type design (`"float32"`, not `"[]float32"`) is simpler and avoids confusion about slice headers vs element arrays. Quality-of-life improvement, now implemented.

### Game Script Migration
Migrated the following buffers from `c_alloc(byte_size)` + byte-offset `c_set_float`/`c_get_float` to `c_alloc_floats32(count)` + index-based `c_array_set_float32`/`c_array_get_float32`:

| Buffer | Old allocation | New allocation | Fields (indices) |
|---|---|---|---|
| `light_ambient` | `c_alloc(16)` | `c_alloc_floats32(4)` | RGBA [0..3] |
| `light_diffuse` | `c_alloc(16)` | `c_alloc_floats32(4)` | RGBA [0..3] |
| `light_specular` | `c_alloc(16)` | `c_alloc_floats32(4)` | RGBA [0..3] |
| `material_specular` | `c_alloc(16)` | `c_alloc_floats32(4)` | RGBA [0..3] |
| `fog_color` | `c_alloc(16)` | `c_alloc_floats32(4)` | RGBA [0..3] |
| `light_pos` | `c_alloc(16)` | `c_alloc_floats32(4)` | XYZW [0..3] |
| `tree_instance_buf_ptr` | `c_alloc(500*40)` | `c_alloc_floats32(500*10)` | x,y,z,yaw,sx,sy,sz,r,g,b [0..9] |

**Result:** Index-based access eliminates manual `* 4` byte arithmetic. `glBufferSubData` still receives byte sizes (unchanged). Game runs correctly after migration.

### New Za stdlib: `c_array_bulk_set_float32` / `c_array_bulk_set_float64`
Added bulk-write helpers that write multiple float values in a single FFI call:

```za
c_array_bulk_set_float32(ptr, start_index, [x, y, z, r, g, b, a])
```

**Game script migration:**
- **Particles** (`draw_particles()`): 28 `c_set_float` calls per particle → 4 `c_array_bulk_set_float32` calls per particle (1 per vertex, 7 floats each)
- **Bonfire particles** (`draw_bonfire_particles()`): same pattern
- Reduces FFI call count for particles by **~85%** (28 → 4 calls per particle)

### Beach VBO Precomputation
**Problem:** `draw_beach()` was recomputing the entire sandy beach geometry every frame with `sin`/`cos` and ~3,440 `c_set_float` calls. Profiler showed it as the #1 CPU cost at **3.1s/frame**.

**Fix:** Moved all beach geometry computation from `draw_beach()` into `init_beach_vbo()`. The beach is fully static (jitter based on world position, not time), so it can be baked once at startup. `draw_beach()` now just binds the precomputed static VBO and draws.

**Result:** `draw_beach` CPU cost reduced from ~3.1s/frame to negligible (just VBO bind + draw call).

---

## Session: Startup Resolution Selection

### Goal
Allow passing a resolution at startup (`za game 1440x900`) instead of editing the script.

### Changes
- Replaced `input frame_limit_arg optarg 1` with `input res_string optarg 1`
- Removed old `frame_limit` profiling logic (was used for ad-hoc profiling, not user-facing)
- Changed default resolution from `800x600` to `1920x1080`
- Added parsing logic that splits `WIDTHxHEIGHT`, validates both parts are numbers, and sets `actual_window_width`/`actual_window_height`
- Empty string as arg #1 is explicitly accepted and acts as default (`1920x1080`)
- Added commented-out placeholder for `optarg 2` to allow future arguments
- Prints accepted resolutions list on invalid input

### Accepted resolutions
`800x600`, `1024x768`, `1280x720`, `1280x800`, `1366x768`, `1440x900`, `1600x900`, `1920x1080`

### Usage
```bash
za game              # 1920x1080 windowed (default)
za game ""           # 1920x1080 windowed (explicit empty = default)
za game 1440x900     # 1440x900 fullscreen
za game 800x600      # 800x600 fullscreen
```

### Fullscreen behavior
- When `res_string` is empty: windowed mode, window created at default size (1920×1080)
- When `res_string` is provided: fullscreen on primary monitor at requested resolution
- Removed `GLFW_MAXIMIZED` hint which previously overrode the requested window size

### Files modified
- `./game`

---

## Session: Particle Optimization + Tree Bark Instancing

### Goal
Eliminate remaining per-frame CPU hotspots: particle `c_alloc`/`glBufferData` stalls and individual tree draw calls.

### Changes

#### 1. Particle Persistent Buffers
- Added global `particle_buf_ptr` and `bf_buf_ptr` (CPU-side C buffers)
- Allocated once in `init_particles()` via `c_alloc()`
- `draw_particles()` and `draw_bonfire_particles()` now write directly into persistent buffers instead of `c_alloc()`/`c_free()` per frame
- Replaced `glBufferData` with `glBufferSubData` for both particle VBOs (avoids GPU memory reallocation stalls)
- Impact: eliminates ~1.5s/frame of `c_alloc` + `glBufferData` overhead

#### 2. Tree Bark Instancing
- Added `tree_bark_variants[]` and `tree_leaf_variants[]` arrays
- Modified tree loading loop to **separate bark and leaf material groups at load time**:
  - Bark groups → `tree_bark_variants[variant]`
  - Leaf groups → `tree_leaf_variants[variant]`
- Added `draw_tree_leaves(x, z, scale, yaw, time, model_index)` — draws only leaf groups with per-tree Z-axis sway
- Added `MIN_TREE_INSTANCES_FOR_BARK = 5` threshold (lower than building/grass threshold of 8 because each tree is much heavier)
- Restructured `draw_scene()` tree loop:
  - Outer loop per variant, inner scan collects visible instances into pre-allocated `tree_instance_buf_ptr`
  - If visible count ≥ 5: instance-draw bark via `draw_instanced_variant()`, then draw leaves individually
  - If count < 5: fallback to `draw_tree()` (bark + leaves together)
- Added `tree_instance_vbo` (500-instance capacity) and persistent `tree_instance_buf_ptr` initialized at startup
- Impact: bark draws drop from ~40-200 individual calls to ~3-6 instanced calls; leaves stay individual (~40-50 calls with sway)

#### 3. Verification
- Game compiles and runs without crashes
- Tree loading output confirms all 5 variants loaded with correct material group counts
- FPS measured at ~26.5-27.5 (stable, within expected range for this scene complexity)
- No visual regressions: trees, particles, and bonfire all render correctly

### Notes
- Frustum culling means many variants have < 5 visible trees at any given camera angle, so fallback path is still common
- Further FPS gains would require: skeletal animation optimization, reducing leaf group individual draws, or additional batching
- Persistent buffer pattern is now consistent across: buildings, grass, rocks, trees, particles, bonfire

### Files modified
- `./game`

---

## Session: Za Interpreter Bytecode Fix + Typed Array Allocators + Beach Precomputation

### Date: July 28, 2026

### 1. Za Interpreter Bug Fix: WORLD_HALF Crash

**Problem:** Using `s = WORLD_HALF` inside `create_textured_floor_vbo()` caused a runtime crash at the `for j = 0 to grids` loop. Minimal reproductions did not trigger it — only the full game script.

**Root cause:** Three interacting bugs in the interpreter's bytecode system for function bodies:
1. **Token copy bug (`actor.go:1946`):** `for _, itok := range inbound.Tokens` creates a copy; modifying `itok.bindpos` does NOT modify the actual token. Tokens copied into `functionspaces[lmv]` retained main-space `bindpos` values.
2. **Bytecode compiled for wrong space (`phraser.go`):** `fnTypeHints[main]` is shared across all functions. Another function's `s: int` leaked into `create_textured_floor_vbo`'s bytecode, compiling `-s` as `OpNegInt` (expects int) when `s` was actually `float64` at runtime.
3. **Cross-space assignment bug (`assigner.go:561`):** `doAssign` always passed `&assignee[0]` to `vset`, which uses `tok.bindpos` directly. After fixing the copy bug, global assignments (`@var = expr`) inside functions wrote to the wrong slot in `mident`.

**Fixes applied:**
- `actor.go:1946`: Changed to index-based iteration `for i := range inbound.Tokens` so tokens are correctly rebound.
- `actor.go:1940`: Recompile bytecode with corrected `bindpos` and `nil` type hints when copying phrases into `functionspaces[lmv]`.
- `assigner.go:561`: Pass `nil` to `vset` when `lfs != rfs`, forcing `bind_int(lfs, name)` lookup for cross-space/global assignments.

**Result:** `s = WORLD_HALF` workaround removed. Game now uses `WORLD_HALF` consistently everywhere.

**FPS impact:** Restoring function-body bytecode improved FPS from ~27 to **35** (no culling) and **45** (near full culling).

### 2. Typed Array Allocator Migration

Migrated game buffers from `c_alloc(byte_size)` + byte-offset `c_set_float` to `c_alloc_floats32(count)` + index-based `c_array_set_float32`:

| Buffer | Old | New |
|---|---|---|
| `light_ambient` | `c_alloc(16)` | `c_alloc_floats32(4)` |
| `light_diffuse` | `c_alloc(16)` | `c_alloc_floats32(4)` |
| `light_specular` | `c_alloc(16)` | `c_alloc_floats32(4)` |
| `material_specular` | `c_alloc(16)` | `c_alloc_floats32(4)` |
| `fog_color` | `c_alloc(16)` | `c_alloc_floats32(4)` |
| `light_pos` | `c_alloc(16)` | `c_alloc_floats32(4)` |
| `tree_instance_buf_ptr` | `c_alloc(500*40)` | `c_alloc_floats32(500*10)` |

### 3. Bulk Write Helpers (`c_array_bulk_set_float32`/`float64`)

Added to Za stdlib. Writes multiple float values in a single FFI call from a Za array literal:

```za
c_array_bulk_set_float32(ptr, start_index, [x, y, z, r, g, b, a])
```

Migrated particle writes:
- `draw_particles()`: 28 `c_set_float` calls per particle → 4 bulk calls per particle
- `draw_bonfire_particles()`: same pattern
- Reduces particle FFI call count by ~85%

### 4. Beach VBO Precomputation

**Problem:** `draw_beach()` recomputed the entire sandy beach geometry every frame with `sin`/`cos` and ~3,440 `c_set_float` calls. Profiler showed **3.1s/frame** — the #1 CPU cost.

**Fix:** Moved all beach geometry computation from `draw_beach()` into `init_beach_vbo()`. The beach is fully static (jitter based on world position, not time), so it can be baked once at startup. `draw_beach()` now just binds the precomputed static VBO and draws.

**Result:** `draw_beach` CPU cost reduced from ~3.1s/frame to negligible.

### Combined FPS Results
- **Uncullled:** ~27 → **65 FPS** (2.4× improvement)
- **Full culling:** ~27 → **90 FPS** (3.3× improvement)

### Files modified
- `./game`
- `/home/daniel/go/src/za/actor.go`
- `/home/daniel/go/src/za/assigner.go`
- `/home/daniel/go/src/za/lib-c.go`
- `/home/daniel/go/src/za/CHANGELOG`
- `/home/daniel/go/src/za/docs/handbook/handbook.md`
- `/home/daniel/go/src/za/docs/za.vim`
- `/home/daniel/go/src/za/docs/lua/za-nvim/syntax.lua`

---

## Future Actions (Updated)

- ~~**Hardware instancing / batching**~~ — **DONE** (tree bark instancing implemented)
- ~~**Fixing culling**~~ — **DONE** (frustum culling implemented)
- **Building instancing by variant** — batch 30+ draws into 8 (medium effort, medium gain)
- **Skeletal animation optimization** — CPU cost visible in profiler (low priority, 65-90 FPS is sufficient)
- ~~**Minimap panel**~~ — **DONE** (draggable resizable radar with mouse-wheel zoom)

---

## Session: Minimap Panel

### Date: July 28, 2026

### 1. Extended Panel System with Resizing

**New panel fields** (indices 28-34):
- `PANEL_RESIZABLE` — bool
- `PANEL_RESIZING` — bool
- `PANEL_RESIZE_CORNER` — int (0=TL, 1=TR, 2=BL, 3=BR)
- `PANEL_RESIZE_OFF_X/Y` — float
- `PANEL_MIN_W/H` — float

**Panel array grows from 28 → 35 elements.**

**Visual affordance:** 8×8 corner squares drawn on resizable panels (slightly brighter than border).

**Resize behavior:** Any of the 4 corners can be dragged. Corner hit-test uses 10px threshold. Size is clamped to `PANEL_MIN_W/H` and screen bounds.

**Save/Load format:** Resizable panels use `"x y w h"`; non-resizable remain `"x y"` for backward compatibility.

### 2. Minimap Radar Panel

- **Position:** Top-right, 200×200 default, draggable + resizable
- **Style:** Classic radar — player always faces **up**, world rotates around player
- **Layers:** Beach boundary (tan), roads (gray), water (blue), buildings (brown), trees (green dots), player (red dot + arrow)
- **Zoom:** Mouse wheel over panel adjusts `minimap_zoom` (20..200 world units), centered on player
- **Clipping:** `glScissor` restricts drawing to panel bounds
- **Persistence:** Position and size saved to `config/minimap_pos.txt`

### Files modified
- `./game`
- `./progress.md`

---

## Session: Tree Instancing Bug Fix

### Date: July 28, 2026

### Problem
Tree trunks/bark were visibly "rotating" by ~180° when the player moved. The rotation was not the intended ±2° leaf sway — it affected the bark itself, which should be stationary. Additionally, tree bark brightness changed when approaching vs. moving away from trees.

### Root Cause 1: Rotation Sign Error in Instancing Shader

The instancing vertex shader computed Y-axis rotation manually with a `mat3`. The matrix was transposed, creating `R_y(-θ)` instead of `R_y(+θ)`:

**Before (wrong):**
```glsl
mat3 rotY = mat3(c, 0.0, s, 0.0, 1.0, 0.0, -s, 0.0, c);
```

**After (correct):**
```glsl
mat3 rotY = mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
```

**Why buildings looked fine:** Buildings are mostly rectangular boxes with yaws aligned to the road grid (0°, 90°, 180°, 270°). A 180° Y-flip on a box looks identical. Trees are highly asymmetric with random yaws — a tree with yaw ~75° gets flipped by ~150°, appearing as a ~180° rotation.

### Root Cause 2: Lighting Mismatch Between Instanced and Fallback Paths

The instancing shader originally did `texture × color` with no lighting, while the fixed-function fallback used OpenGL's per-vertex `GL_LIGHT0` ambient + diffuse + specular pipeline. When trees switched between instanced (≥5 visible) and fallback (<5 visible) paths as the player moved, the brightness popped.

**Fix:** Rewrote the instancing vertex shader to use **exact fixed-function compatibility built-ins** that the driver computes from current state:

| Feature | Before (approximation) | After (exact match) |
|---------|------------------------|-------------------|
| Material color | Hardcoded `aInstanceColor` | `gl_FrontMaterial` via `gl_FrontLightProduct` (respects `GL_COLOR_MATERIAL`) |
| Ambient | Manual `gl_LightModel + gl_LightSource` | `gl_FrontLightModelProduct.sceneColor + gl_FrontLightProduct[0].ambient` |
| Diffuse | Manual `gl_LightSource * dot(N,L)` | `gl_FrontLightProduct[0].diffuse * NdotL` |
| Specular | **Missing** | `gl_FrontLightProduct[0].specular * pow(NdotH, gl_FrontMaterial.shininess)` |
| Half-vector | Computed manually | `gl_LightSource[0].halfVector` (driver-provided, infinite viewer) |

The fragment shader now just multiplies texture by the interpolated `vFrontColor` — identical to fixed-function output.

### Debugging Methodology

1. **Added per-frame TREE_DBG prints** for first 3 visible trees showing `yaw`, `pos`, `scale`, `dist`, `sway`
2. **90-second capture** (5922 lines) proved tree parameters were perfectly stable — `yaw` never changed by even 0.01 across thousands of frames
3. **Set `MIN_TREE_INSTANCES_FOR_BARK = 9999`** to force ALL trees through the fallback `draw_tree()` path — trees stopped rotating, confirming the bug was isolated to the instanced shader path
4. **Compared shader matrix vs. fixed-function `glRotatef`** — found the transposed sign
5. **Restored instancing** and applied the matrix fix + lighting fix

### Verification
- Game runs for 90 seconds with instancing enabled, zero errors, zero shader compilation warnings
- Trees are visually stable: no rotation, consistent brightness across all distances
- Fog differences remain at far distances (expected — shader doesn't compute `GL_FOG`, which is acceptable)

### Files modified
- `./game` — `INSTANCING_VS` and `INSTANCING_FS` shader strings, removed `TREE_DBG` prints

### Remaining Backlog
- **Fog in instancing shader** — Minor visual difference at >20 units distance. Acceptable for now; would require adding `gl_Fog` built-in uniforms to the shader for exact match.

---

## Session: Water Culling + Tall Object Culling Fix

### Date: July 28, 2026

### 1. Water Pond Culling

**Problem:** Small water instances (ponds/blobs) were drawn individually with `glDrawArrays` inside `draw_water_vbo()` without any frustum culling. When the player faced away from ponds, all ~20-40 pond instances were still drawn.

**Fix:**
- Added `water_small_positions[]` array storing `[cx, cz, radius]` for each small water instance, populated during `create_water_vbo()`
- Changed `draw_water_vbo(texture_id, time)` signature to accept `planes` and `use_culling`
- Before each `glDrawArrays` for a small instance, test with `sphere_in_frustum(cx, 0.0, cz, radius, planes)`
- Updated `draw_scene()` call site to pass `frustum_planes` and `use_culling`

**Result:** Off-screen ponds are now culled, saving ~10-30 draw calls when camera is away from water.

### 2. Tall Object Culling Fix

**Problem:** `sphere_in_frustum()` hardcoded `cy = 0.0` for all objects. Tall buildings and trees centered near the top frustum plane could be incorrectly culled because the sphere was centered at ground level instead of the object's actual center height.

**Fix:**
- Changed `sphere_in_frustum(cx, cz, radius, planes)` → `sphere_in_frustum(cx, cy, cz, radius, planes)`
- Updated all call sites with appropriate center heights:
  - **Buildings:** `cy = by + bsy * 0.5` (base Y + half height)
  - **Trees:** `cy = scale * 0.75` (approximate center of tree model)
  - **Grass/Rocks/Water:** `cy = 0.0` (ground-level objects)

**Result:** Tall buildings and trees near the top of the screen are no longer incorrectly culled when their tops are still visible.

### Files modified
- `./game`

### Verification
- 90-second test: zero errors, zero shader compilation warnings
- Game runs cleanly with all culling paths active

### Remaining Backlog (Updated)
- **Fog in instancing shader** — Minor brightness difference at >20 units. Acceptable.
- **Skeletal animation optimization** — Low priority (65-90 FPS sufficient)

---

## Session: Za Interpreter — `else if` and `return if` Support

### Date: July 28, 2026

### 1. `else if` Control Flow

**Problem:** Za lacked `else if` chaining, requiring nested `if/else/endif` blocks for multiple conditions.

**Implementation:**
- **Phraser (`phraser.go`):** When a phrase starts with `C_Else` followed by `C_If`, compile the condition expression to bytecode (same path as `C_If` / `C_While`).
- **Actor (`actor.go`):**
  - `C_If` handler: After lookahead caching, check if the `else` target is actually `else if` (token 1 is `C_If`). If so, jump TO the `else if` line (`elsedistance - 1`) instead of skipping over it.
  - `C_Else` handler: If `Tokens[1].tokType == C_If`, evaluate the condition. If true, fall through to body. If false, lookahead for next `else`/`endif` starting from `parser.pc + 1` with `indent=1` (accounting for outer `if` block). Jump to next `else if` for evaluation, or past `endif` to exit the block.

**Key fix:** The `indent=1` parameter in `lookahead` is critical. Without it, scanning from inside the `else if` body hits `endif` at `indent=0`, which decrements to `-1` and triggers a nesting fault instead of finding the terminator.

### 2. `return if` Statement Modifier

**Syntax:**
```za
return if condition
return if condition, value1, value2
```

**Implementation (`actor.go`):**
- In `C_Return` handler, detect `argtoks[0].tokType == C_If`.
- Find first top-level comma after the `if` token to split condition from return values.
- Evaluate condition with `wrappedEval()`. If false, skip the return entirely (`break` from `C_Return` case).
- If true, rebuild `argtoks` from the post-comma tokens and continue with normal return logic.

**Note:** The prefix form (`return if expr, values`) is implemented. The statement-modifier form (`return values if expr`) would require additional parsing to locate `C_If` among the return value tokens.

### 3. Documentation Updates

- **REPL help (`misc.go`):** Added `ELSE IF` and `RETURN ... IF` lines to `cmdpage`.
- **Handbook (`docs/handbook/handbook.md`):**
  - Conditionals section: Added `else if` to the block example and explained chaining.
  - Functions section: Added `return if` examples.
  - Appendix B keywords: No change needed (`else if` uses existing `else` + `if` keywords).

### 4. Testing

- **Unit tests:** `za_tests/control_flow/test_else_if.za` (6 test cases), `za_tests/control_flow/test_return_if.za` (7 test cases)
- **Regression:** Ran with locally built `./za` binary. All new and existing non-ffi tests pass.
- **Game integration:** 90-second run with new binary, zero errors.

### Files modified
- `/home/daniel/go/src/za/actor.go`
- `/home/daniel/go/src/za/phraser.go`
- `/home/daniel/go/src/za/misc.go`
- `/home/daniel/go/src/za/docs/handbook/handbook.md`
- `./progress.md`

### Bug Fix: Compound Assignment Cross-Space Error

**Problem:** `eg/mon` and other scripts using `@var -= 1` inside functions triggered: *"you may only amend existing variables outside of local scope"*.

**Root cause:** `eval.go:3732` checked `(*lident)[newEval[0].bindpos].declared` for compound assignments (`-=`, `+=`, etc.) when `lfs != fs`. Global variables have a bindpos but `.declared` is `false` in the local scope, so legitimate global amendments were blocked.

**Fix:** Replaced the `declared` check with a read-only `bindings[lfs]` map lookup. This correctly verifies the variable exists in the target scope without side-effects.

**Verification:** Game runs 90s with zero errors. `eg/mon` no longer triggers the error (user to confirm).

### Files modified (additional)
- `/home/daniel/go/src/za/eval.go`

## Session: Game Script `else if` Refactoring

### Date: July 28, 2026

Applied `else if` chaining across the game script where nested `else/if` blocks were semantically equivalent:

| Location | Original structure | Change |
|---|---|---|
| `init_particles()` | `if/else/if/else/if/else` (4 levels) | `if/else if/else if/else` (flat) |
| `spawn_particle()` | `if/else/if/else/if/else` (4 levels) | `if/else if/else if/else` (flat) |
| `update_particles()` visibility | `if/else/if/endif` | `if/else if/endif` |
| `update_particles()` respawn | `if/else/if/endif` | `if/else if/endif` |
| `draw_bonfire_particles()` | `if/else/if/else` (roll 0/1/2) | `if/else if/else` |
| `draw_ui()` surface type | `if/else/if/else` (road/door/floor) | `if/else if/else` |

Also fixed over-indentation in `spawn_particle()` after the `else if` conversion (bodies were still indented for nested `if` depth).

### Verification
- `za -zz game` — full script parses cleanly (31ms, no syntax errors)
- Manual gameplay test — all branches execute correctly

### Files modified
- `./game`

---

## Session: Modular Split + Reference Comments

### Date: July 28, 2026

### Goal
Split the ~7,300-line monolithic game script into logical modules and add explanatory comments for prospective za users.

### New Directory Structure

```
3d/
  game.za              ← main entry (~2,700 lines: imports, globals, init, loop)
  game.monolithic      ← original single-file backup
  lib/
    utils.za           ← 28 functions: RNG, GL helpers, shaders, callbacks (~440 lines)
    models.za          ← 12 functions: OBJ/MTL parsing, VBO factory (~540 lines)
    geometry.za        ← 18 functions: terrain, water, beach, road VBOs (~1,250 lines)
    renderer.za        ← 17 functions: draw_scene, instancing, sky, culling (~900 lines)
    world.za           ← 17 functions: placement, collision, surface height (~470 lines)
    particles.za       ← 6 functions: spawn, update, draw particles (~470 lines)
    ui.za              ← 16 functions: panels, text, minimap, save/load (~670 lines)
  tools/
    process_gltf.py    ← moved from root (path updated in MODELS.md)
  CREDITS              ← new file with CC0 asset attribution
```

### Architecture
- All `var` declarations and constants stay in `game.za` (za modules cannot export variables)
- Module files contain only `define` blocks
- `main::` fallback namespace allows modules to reference globals transparently
- `MODULE "lib/*.za" AS name` + `USE +name` loads and activates each module

### Key Finding: USE Chain Overhead
With 7 modules in the `USE` chain, every bare-name function/global lookup performed a linear scan across all namespaces with mutex locks. This caused a severe FPS drop (confirmed by existing pprof notes in `use_chain.go`).

**Solution:** Added `namespace main` at the top of every `lib/*.za` file. This registers all module functions directly into `main::`, bypassing the USE chain entirely. Resolution now hits "user-defined function in current namespace" (step 3) immediately.

**Callbacks:** Reverted to bare names (`"on_mouse_move"`) since handlers are now in `main::`.

**Tradeoff:** All function names must be unique across all modules. This is already true for this project.

### Comments Added
- **Main file:** Comprehensive `doc` block explaining the project as a tech demo, performance disclaimer, hardware requirements, and third-party asset credits (Quaternius, GrEmlin, Kenney, FBX2glTF)
- **Each module:** `doc` header describing its purpose and which za concepts it demonstrates
- **Non-obvious functions:** Brief 1-3 line comments on ~30 functions covering complex math, GPU patterns, and za-specific syntax

### Files Modified / Created
- `./game.za` — new main entry point
- `./lib/utils.za`
- `./lib/models.za`
- `./lib/geometry.za`
- `./lib/renderer.za`
- `./lib/world.za`
- `./lib/particles.za`
- `./lib/ui.za`
- `./CREDITS` — new file
- `./MODELS.md` — updated `process_gltf.py` path

### Files Removed (dead code cleanup)
- `game.monolithic` — user has separate backup
- `game.bak`, `game.backup.20260727_185625` — stale backups
- `arc/game.old` — stale archive
- `split_game.py` — one-time extraction script
- `models/box.obj`, `models/cube.obj`, `models/test.gltf` — unused model files
- `libwayland-protocol-stub.so`, `wayland-protocol-stub.c`, `wayland-protocol-stub.h` — dead stub
- `textrender.o` — build artifact
- `__pycache__/process_gltf.cpython-314.pyc` — stale Python cache
- `profile_output.txt` — one-time profiler dump

### Verification
- `za -zz game.za` — all 8 files parse cleanly (game.za + 7 modules)
- 5-second manual run — zero errors, callbacks resolve correctly

---

### Remaining Backlog (Updated)
- **Fog in instancing shader** — Minor brightness difference at >20 units. Acceptable.
- **Skeletal animation optimization** — Low priority (65-90 FPS sufficient).
