#!/usr/bin/env python3
"""
process_gltf.py - Convert Quaternius FBX2glTF characters to custom .skl binary format.

Reads a glTF 2.0 file + buffer.bin and writes a .skl binary that libskel.so loads.

.skl binary format (SKL1):
  Header: magic(4) num_verts(4) num_indices(4) num_joints(4) num_animations(4) max_keyframes(4) num_nodes(4)
  Mesh:   positions[verts*3] normals[verts*3] uvs[verts*2] joints[verts*4 shorts] weights[verts*4] indices[indices shorts]
  Skin:   joint_nodes[joints] inverse_bind_matrices[joints*16]
  MeshNodeGlobal: mesh_node_global[16]
  DefaultTRS: default_t[num_nodes*3] default_r[num_nodes*4] default_s[num_nodes*3]
  Hierarchy: num_nodes(4) node_parents[num_nodes]
  Animations: for each anim { name(64) num_channels(4) for each ch { target_node(4) target_path(4) num_keyframes(4) times[kf] values[kf*3or4] } }
"""
import json
import struct
import sys
import os
import math


def read_buffer_view(data, buffer_views, accessors, accessor_idx):
    """Read accessor data from buffer."""
    acc = accessors[accessor_idx]
    bv = buffer_views[acc["bufferView"]]
    offset = bv["byteOffset"] + acc.get("byteOffset", 0)
    count = acc["count"]
    comp_type = acc["componentType"]
    type_str = acc["type"]

    type_sizes = {"SCALAR": 1, "VEC2": 2, "VEC3": 3, "VEC4": 4, "MAT4": 16}
    elem_size = type_sizes[type_str]

    if comp_type == 5126:  # FLOAT
        return list(struct.unpack_from(f"<{count * elem_size}f", data, offset))
    elif comp_type == 5123:  # UNSIGNED_SHORT
        raw = list(struct.unpack_from(f"<{count * elem_size}H", data, offset))
        return raw
    elif comp_type == 5125:  # UNSIGNED_INT
        return list(struct.unpack_from(f"<{count * elem_size}I", data, offset))
    else:
        raise ValueError(f"Unknown componentType {comp_type}")


def mat4_mul(a, b):
    """Multiply two 4x4 matrices (column-major, 16 floats)."""
    out = [0.0] * 16
    for i in range(4):
        for j in range(4):
            s = 0.0
            for k in range(4):
                s += a[k * 4 + i] * b[j * 4 + k]
            out[j * 4 + i] = s
    return out


def node_local_matrix(node):
    """Compute local 4x4 matrix from node TRS."""
    t = node.get("translation", [0, 0, 0])
    r = node.get("rotation", [0, 0, 0, 1])
    s = node.get("scale", [1, 1, 1])
    x, y, z, w = r
    xx, yy, zz = x * x, y * y, z * z
    xy, xz, yz = x * y, x * z, y * z
    wx, wy, wz = w * x, w * y, w * z
    m = [0.0] * 16
    m[0] = (1 - 2 * (yy + zz)) * s[0]
    m[1] = (2 * (xy + wz)) * s[0]
    m[2] = (2 * (xz - wy)) * s[0]
    m[3] = 0.0
    m[4] = (2 * (xy - wz)) * s[1]
    m[5] = (1 - 2 * (xx + zz)) * s[1]
    m[6] = (2 * (yz + wx)) * s[1]
    m[7] = 0.0
    m[8] = (2 * (xz + wy)) * s[2]
    m[9] = (2 * (yz - wx)) * s[2]
    m[10] = (1 - 2 * (xx + yy)) * s[2]
    m[11] = 0.0
    m[12] = t[0]
    m[13] = t[1]
    m[14] = t[2]
    m[15] = 1.0
    return m


def process(gltf_path, out_path=None):
    gltf_dir = os.path.dirname(gltf_path)
    with open(gltf_path, "r") as f:
        gltf = json.load(f)

    # Load buffer
    buf_info = gltf["buffers"][0]
    buf_uri = buf_info.get("uri", "buffer.bin")
    if buf_uri.startswith("data:"):
        import base64
        raw = base64.b64decode(buf_uri.split(",", 1)[1])
    else:
        with open(os.path.join(gltf_dir, buf_uri), "rb") as f:
            raw = f.read()

    bvs = gltf["bufferViews"]
    accs = gltf["accessors"]
    nodes = gltf["nodes"]
    num_nodes = len(nodes)

    # Build parent array
    node_parents = [-1] * num_nodes
    for i, node in enumerate(nodes):
        for child in node.get("children", []):
            node_parents[child] = i

    # Find the skinned mesh (has JOINTS_0 attribute)
    skin_data = gltf.get("skins", [{}])[0]
    joint_node_ids = skin_data["joints"]
    num_joints = len(joint_node_ids)
    joint_node_to_idx = {n: i for i, n in enumerate(joint_node_ids)}

    # Read inverse bind matrices
    ibm_acc_idx = skin_data["inverseBindMatrices"]
    ibm_data = read_buffer_view(raw, bvs, accs, ibm_acc_idx)
    # ibm_data is flat list of num_joints * 16 floats

    # Find skinned mesh node
    skinned_mesh_node = None
    for i, node in enumerate(nodes):
        if "skin" in node:
            skinned_mesh_node = i
            break
    if skinned_mesh_node is None:
        print("ERROR: No skinned mesh found")
        sys.exit(1)

    # Find the mesh with JOINTS_0
    mesh_idx = nodes[skinned_mesh_node]["mesh"]
    prim = gltf["meshes"][mesh_idx]["primitives"][0]
    attrs = prim["attributes"]

    pos_data = read_buffer_view(raw, bvs, accs, attrs["POSITION"])
    norm_data = read_buffer_view(raw, bvs, accs, attrs["NORMAL"])
    uv_data = read_buffer_view(raw, bvs, accs, attrs["TEXCOORD_0"])
    joint_data = read_buffer_view(raw, bvs, accs, attrs["JOINTS_0"])
    weight_data = read_buffer_view(raw, bvs, accs, attrs["WEIGHTS_0"])
    idx_data = read_buffer_view(raw, bvs, accs, prim["indices"])

    num_verts = len(pos_data) // 3
    num_indices = len(idx_data)

    # JOINTS_0 values are already skin joint indices (0..num_joints-1), pass through as-is
    remapped_joints = joint_data

    # Compute node world transforms for mesh_node_global
    node_world = [None] * num_nodes
    for i in range(num_nodes):
        local = node_local_matrix(nodes[i])
        p = node_parents[i]
        if p >= 0 and node_world[p] is not None:
            node_world[i] = mat4_mul(node_world[p], local)
        else:
            node_world[i] = local

    mesh_node_global = node_world[skinned_mesh_node]

    # Default TRS for all nodes (bind pose)
    default_t = []
    default_r = []
    default_s = []
    for i in range(num_nodes):
        node = nodes[i]
        t = node.get("translation", [0.0, 0.0, 0.0])
        r = node.get("rotation", [0.0, 0.0, 0.0, 1.0])
        s = node.get("scale", [1.0, 1.0, 1.0])
        default_t.extend(t)
        default_r.extend(r)
        default_s.extend(s)

    # Parse animations
    animations = gltf.get("animations", [])
    anim_list = []
    max_kf = 0
    for anim in animations:
        name = anim.get("name", "unnamed")[:63]
        samplers = {}
        for si, samp in enumerate(anim.get("samplers", [])):
            input_acc = accs[samp["input"]]
            output_acc = accs[samp["output"]]
            times = read_buffer_view(raw, bvs, accs, samp["input"])
            values = read_buffer_view(raw, bvs, accs, samp["output"])
            samplers[si] = (times, values, output_acc["type"])

        channels = []
        for ch in anim.get("channels", []):
            sampler_idx = ch["sampler"]
            target = ch["target"]
            node = target["node"]
            path = target["path"]
            path_id = {"translation": 0, "rotation": 1, "scale": 2}[path]
            times, values, vtype = samplers[sampler_idx]
            num_kf = len(times)
            if num_kf > max_kf:
                max_kf = num_kf
            channels.append((node, path_id, times, values))
        anim_list.append((name, channels))

    if out_path is None:
        base = os.path.splitext(os.path.basename(gltf_path))[0]
        out_path = os.path.join(os.path.dirname(gltf_path), "..", base + ".skl")
        out_path = os.path.normpath(out_path)

    with open(out_path, "wb") as f:
        def wi(v):
            f.write(struct.pack("<i", v))
        def wf(v):
            f.write(struct.pack("<f", v))
        def wh(v):
            f.write(struct.pack("<H", v))

        f.write(b"SKL1")
        wi(num_verts)
        wi(num_indices)
        wi(num_joints)
        wi(len(anim_list))
        wi(max_kf)
        wi(num_nodes)

        # Mesh data
        for v in pos_data:
            wf(v)
        for v in norm_data:
            wf(v)
        for v in uv_data:
            wf(v)
        for v in remapped_joints:
            wh(v)
        for v in weight_data:
            wf(v)
        for v in idx_data:
            wh(v)

        # Skin data
        for nid in joint_node_ids:
            wi(nid)
        for v in ibm_data:
            wf(v)

        # Mesh node global transform
        for v in mesh_node_global:
            wf(v)

        # Default TRS for all nodes (bind pose)
        for v in default_t:
            wf(v)
        for v in default_r:
            wf(v)
        for v in default_s:
            wf(v)

        # Node hierarchy
        wi(num_nodes)
        for p in node_parents:
            wi(p)

        # Animations
        for name, channels in anim_list:
            name_bytes = name.encode("utf-8")[:63]
            name_bytes = name_bytes + b"\x00" * (64 - len(name_bytes))
            f.write(name_bytes)
            wi(len(channels))
            for target_node, path_id, times, values in channels:
                wi(target_node)
                wi(path_id)
                nkf = len(times)
                wi(nkf)
                for t in times:
                    wf(t)
                for v in values:
                    wf(v)

    print(f"Wrote {out_path}")
    print(f"  verts={num_verts} indices={num_indices} joints={num_joints} nodes={num_nodes} anims={len(anim_list)}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <gltf_file>")
        sys.exit(1)
    for path in sys.argv[1:]:
        process(path)
