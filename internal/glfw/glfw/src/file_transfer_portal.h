//========================================================================
// GLFW 3.4 FileTransfer portal support - www.glfw.org
//------------------------------------------------------------------------
// Copyright (c) 2026 GLFW contributors
//
// This software is provided 'as-is', without any express or implied
// warranty. In no event will the authors be held liable for any damages
// arising from the use of this software.
//
// Permission is granted to anyone to use this software for any purpose,
// including commercial applications, and to alter it and redistribute it
// freely, subject to the following restrictions:
//
// 1. The origin of this software must not be misrepresented; you must not
//    claim that you wrote the original software. If you use this software
//    in a product, an acknowledgment in the product documentation would
//    be appreciated but is not required.
//
// 2. Altered source versions must be plainly marked as such, and must not
//    be misrepresented as being the original software.
//
// 3. This notice may not be removed or altered from any source
//    distribution.
//
//========================================================================

#pragma once

#if defined(_GLFW_X11) || defined(_GLFW_WAYLAND)

#define FILE_TRANSFER_PORTAL_MIME_TYPE "application/vnd.portal.filetransfer"

#define DBUS_FALSE 0
#define DBUS_BUS_SESSION 0
#define DBUS_TYPE_STRING ((int) 's')
#define DBUS_TYPE_ARRAY ((int) 'a')
#define DBUS_TIMEOUT_USE_DEFAULT (-1)

typedef uint32_t dbus_bool_t;

typedef struct DBusError
{
    const char* name;
    const char* message;
    unsigned int dummy1 : 1;
    unsigned int dummy2 : 1;
    unsigned int dummy3 : 1;
    unsigned int dummy4 : 1;
    unsigned int dummy5 : 1;
    void* padding1;
} DBusError;

typedef struct DBusConnection DBusConnection;
typedef struct DBusMessage DBusMessage;

typedef struct DBusMessageIter
{
#if UINTPTR_MAX > UINT64_MAX
    void* dummy[16];
#else
    void* dummy1;
    void* dummy2;
    uint32_t dummy3;
    int dummy4;
    int dummy5;
    int dummy6;
    int dummy7;
    int dummy8;
    int dummy9;
    int dummy10;
    int dummy11;
    int pad1;
    void* pad2;
    void* pad3;
#endif
} DBusMessageIter;

typedef void (* PFN_dbus_error_init)(DBusError*);
typedef void (* PFN_dbus_error_free)(DBusError*);
typedef dbus_bool_t (* PFN_dbus_error_is_set)(const DBusError*);
typedef DBusConnection* (* PFN_dbus_bus_get)(int,DBusError*);
typedef void (* PFN_dbus_connection_unref)(DBusConnection*);
typedef void (* PFN_dbus_connection_set_exit_on_disconnect)(DBusConnection*,dbus_bool_t);
typedef DBusMessage* (* PFN_dbus_connection_send_with_reply_and_block)(DBusConnection*,DBusMessage*,int,DBusError*);
typedef DBusMessage* (* PFN_dbus_message_new_method_call)(const char*,const char*,const char*,const char*);
typedef void (* PFN_dbus_message_unref)(DBusMessage*);
typedef dbus_bool_t (* PFN_dbus_message_iter_init)(DBusMessage*,DBusMessageIter*);
typedef void (* PFN_dbus_message_iter_init_append)(DBusMessage*,DBusMessageIter*);
typedef dbus_bool_t (* PFN_dbus_message_iter_append_basic)(DBusMessageIter*,int,const void*);
typedef dbus_bool_t (* PFN_dbus_message_iter_open_container)(DBusMessageIter*,int,const char*,DBusMessageIter*);
typedef dbus_bool_t (* PFN_dbus_message_iter_close_container)(DBusMessageIter*,DBusMessageIter*);
typedef int (* PFN_dbus_message_iter_get_arg_type)(DBusMessageIter*);
typedef int (* PFN_dbus_message_iter_get_element_type)(DBusMessageIter*);
typedef void (* PFN_dbus_message_iter_recurse)(DBusMessageIter*,DBusMessageIter*);
typedef int (* PFN_dbus_message_iter_get_element_count)(DBusMessageIter*);
typedef void (* PFN_dbus_message_iter_get_basic)(DBusMessageIter*,void*);
typedef dbus_bool_t (* PFN_dbus_message_iter_next)(DBusMessageIter*);

typedef struct _GLFWfileTransferPortal
{
    void* handle;
    PFN_dbus_error_init error_init;
    PFN_dbus_error_free error_free;
    PFN_dbus_error_is_set error_is_set;
    PFN_dbus_bus_get bus_get;
    PFN_dbus_connection_unref connection_unref;
    PFN_dbus_connection_set_exit_on_disconnect connection_set_exit_on_disconnect;
    PFN_dbus_connection_send_with_reply_and_block connection_send_with_reply_and_block;
    PFN_dbus_message_new_method_call message_new_method_call;
    PFN_dbus_message_unref message_unref;
    PFN_dbus_message_iter_init message_iter_init;
    PFN_dbus_message_iter_init_append message_iter_init_append;
    PFN_dbus_message_iter_append_basic message_iter_append_basic;
    PFN_dbus_message_iter_open_container message_iter_open_container;
    PFN_dbus_message_iter_close_container message_iter_close_container;
    PFN_dbus_message_iter_get_arg_type message_iter_get_arg_type;
    PFN_dbus_message_iter_get_element_type message_iter_get_element_type;
    PFN_dbus_message_iter_recurse message_iter_recurse;
    PFN_dbus_message_iter_get_element_count message_iter_get_element_count;
    PFN_dbus_message_iter_get_basic message_iter_get_basic;
    PFN_dbus_message_iter_next message_iter_next;
} _GLFWfileTransferPortal;

#define dbus_error_init _glfw.fileTransferPortal.error_init
#define dbus_error_free _glfw.fileTransferPortal.error_free
#define dbus_error_is_set _glfw.fileTransferPortal.error_is_set
#define dbus_bus_get _glfw.fileTransferPortal.bus_get
#define dbus_connection_unref _glfw.fileTransferPortal.connection_unref
#define dbus_connection_set_exit_on_disconnect _glfw.fileTransferPortal.connection_set_exit_on_disconnect
#define dbus_connection_send_with_reply_and_block _glfw.fileTransferPortal.connection_send_with_reply_and_block
#define dbus_message_new_method_call _glfw.fileTransferPortal.message_new_method_call
#define dbus_message_unref _glfw.fileTransferPortal.message_unref
#define dbus_message_iter_init _glfw.fileTransferPortal.message_iter_init
#define dbus_message_iter_init_append _glfw.fileTransferPortal.message_iter_init_append
#define dbus_message_iter_append_basic _glfw.fileTransferPortal.message_iter_append_basic
#define dbus_message_iter_open_container _glfw.fileTransferPortal.message_iter_open_container
#define dbus_message_iter_close_container _glfw.fileTransferPortal.message_iter_close_container
#define dbus_message_iter_get_arg_type _glfw.fileTransferPortal.message_iter_get_arg_type
#define dbus_message_iter_get_element_type _glfw.fileTransferPortal.message_iter_get_element_type
#define dbus_message_iter_recurse _glfw.fileTransferPortal.message_iter_recurse
#define dbus_message_iter_get_element_count _glfw.fileTransferPortal.message_iter_get_element_count
#define dbus_message_iter_get_basic _glfw.fileTransferPortal.message_iter_get_basic
#define dbus_message_iter_next _glfw.fileTransferPortal.message_iter_next

void _glfwInitFileTransferPortal(void);
void _glfwTerminateFileTransferPortal(void);
void _glfwInputFileTransferPortalDrop(_GLFWwindow* window, const char* key);

#endif // _GLFW_X11 || _GLFW_WAYLAND
