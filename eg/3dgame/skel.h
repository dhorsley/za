#ifndef SKEL_H
#define SKEL_H

#ifdef __cplusplus
extern "C" {
#endif

void* skel_load(const char* path);
void skel_animate(void* handle, const char* anim_name, float time);
void skel_draw(void* handle, float* mvp);
float skel_get_anim_duration(void* handle, const char* anim_name);

#ifdef __cplusplus
}
#endif

#endif
