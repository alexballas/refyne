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

#include "internal.h"

#if defined(_GLFW_X11) || defined(_GLFW_WAYLAND)

#include <assert.h>
#include <string.h>

void _glfwInitFileTransferPortal(void)
{
    if (_glfw.fileTransferPortal.handle)
        return;

    _glfw.fileTransferPortal.handle = _glfwPlatformLoadModule("libdbus-1.so.3");
    if (!_glfw.fileTransferPortal.handle)
        return;

    _glfw.fileTransferPortal.error_init = (PFN_dbus_error_init)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_error_init");
    _glfw.fileTransferPortal.error_free = (PFN_dbus_error_free)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_error_free");
    _glfw.fileTransferPortal.error_is_set = (PFN_dbus_error_is_set)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_error_is_set");
    _glfw.fileTransferPortal.bus_get = (PFN_dbus_bus_get)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_bus_get");
    _glfw.fileTransferPortal.connection_unref = (PFN_dbus_connection_unref)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_connection_unref");
    _glfw.fileTransferPortal.connection_set_exit_on_disconnect = (PFN_dbus_connection_set_exit_on_disconnect)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_connection_set_exit_on_disconnect");
    _glfw.fileTransferPortal.connection_send_with_reply_and_block = (PFN_dbus_connection_send_with_reply_and_block)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_connection_send_with_reply_and_block");
    _glfw.fileTransferPortal.message_new_method_call = (PFN_dbus_message_new_method_call)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_new_method_call");
    _glfw.fileTransferPortal.message_unref = (PFN_dbus_message_unref)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_unref");
    _glfw.fileTransferPortal.message_iter_init = (PFN_dbus_message_iter_init)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_init");
    _glfw.fileTransferPortal.message_iter_init_append = (PFN_dbus_message_iter_init_append)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_init_append");
    _glfw.fileTransferPortal.message_iter_append_basic = (PFN_dbus_message_iter_append_basic)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_append_basic");
    _glfw.fileTransferPortal.message_iter_open_container = (PFN_dbus_message_iter_open_container)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_open_container");
    _glfw.fileTransferPortal.message_iter_close_container = (PFN_dbus_message_iter_close_container)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_close_container");
    _glfw.fileTransferPortal.message_iter_get_arg_type = (PFN_dbus_message_iter_get_arg_type)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_get_arg_type");
    _glfw.fileTransferPortal.message_iter_get_element_type = (PFN_dbus_message_iter_get_element_type)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_get_element_type");
    _glfw.fileTransferPortal.message_iter_recurse = (PFN_dbus_message_iter_recurse)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_recurse");
    _glfw.fileTransferPortal.message_iter_get_element_count = (PFN_dbus_message_iter_get_element_count)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_get_element_count");
    _glfw.fileTransferPortal.message_iter_get_basic = (PFN_dbus_message_iter_get_basic)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_get_basic");
    _glfw.fileTransferPortal.message_iter_next = (PFN_dbus_message_iter_next)
        _glfwPlatformGetModuleSymbol(_glfw.fileTransferPortal.handle, "dbus_message_iter_next");

    if (!_glfw.fileTransferPortal.error_init ||
        !_glfw.fileTransferPortal.error_free ||
        !_glfw.fileTransferPortal.error_is_set ||
        !_glfw.fileTransferPortal.bus_get ||
        !_glfw.fileTransferPortal.connection_unref ||
        !_glfw.fileTransferPortal.connection_set_exit_on_disconnect ||
        !_glfw.fileTransferPortal.connection_send_with_reply_and_block ||
        !_glfw.fileTransferPortal.message_new_method_call ||
        !_glfw.fileTransferPortal.message_unref ||
        !_glfw.fileTransferPortal.message_iter_init ||
        !_glfw.fileTransferPortal.message_iter_init_append ||
        !_glfw.fileTransferPortal.message_iter_append_basic ||
        !_glfw.fileTransferPortal.message_iter_open_container ||
        !_glfw.fileTransferPortal.message_iter_close_container ||
        !_glfw.fileTransferPortal.message_iter_get_arg_type ||
        !_glfw.fileTransferPortal.message_iter_get_element_type ||
        !_glfw.fileTransferPortal.message_iter_recurse ||
        !_glfw.fileTransferPortal.message_iter_get_element_count ||
        !_glfw.fileTransferPortal.message_iter_get_basic ||
        !_glfw.fileTransferPortal.message_iter_next)
    {
        _glfwTerminateFileTransferPortal();
    }
}

void _glfwTerminateFileTransferPortal(void)
{
    if (_glfw.fileTransferPortal.handle)
        _glfwPlatformFreeModule(_glfw.fileTransferPortal.handle);

    memset(&_glfw.fileTransferPortal, 0, sizeof(_glfw.fileTransferPortal));
}

void _glfwInputFileTransferPortalDrop(_GLFWwindow* window, const char* key)
{
    DBusConnection* connection = NULL;
    DBusMessage* message = NULL;
    DBusMessage* reply = NULL;
    char** paths = NULL;
    DBusError error;

    assert(_glfw.fileTransferPortal.handle != NULL);
    assert(window != NULL);
    assert(key != NULL);

    dbus_error_init(&error);

    connection = dbus_bus_get(DBUS_BUS_SESSION, &error);
    if (dbus_error_is_set(&error))
    {
        _glfwInputError(GLFW_PLATFORM_ERROR, "DBus: %s", error.message);
        goto cleanup;
    }
    if (!connection)
    {
        _glfwInputError(GLFW_PLATFORM_ERROR,
                        "DBus: Failed to connect to the session bus");
        goto cleanup;
    }

    dbus_connection_set_exit_on_disconnect(connection, DBUS_FALSE);

    message = dbus_message_new_method_call(
        "org.freedesktop.portal.Documents",
        "/org/freedesktop/portal/documents",
        "org.freedesktop.portal.FileTransfer",
        "RetrieveFiles");
    if (!message)
    {
        _glfwInputError(GLFW_OUT_OF_MEMORY, NULL);
        goto cleanup;
    }

    DBusMessageIter args, options;
    dbus_message_iter_init_append(message, &args);
    if (!dbus_message_iter_append_basic(&args, DBUS_TYPE_STRING, &key) ||
        !dbus_message_iter_open_container(&args, DBUS_TYPE_ARRAY, "{sv}", &options) ||
        !dbus_message_iter_close_container(&args, &options))
    {
        _glfwInputError(GLFW_OUT_OF_MEMORY, NULL);
        goto cleanup;
    }

    reply = dbus_connection_send_with_reply_and_block(
        connection, message, DBUS_TIMEOUT_USE_DEFAULT, &error);
    if (dbus_error_is_set(&error))
    {
        _glfwInputError(GLFW_PLATFORM_ERROR, "DBus: %s", error.message);
        goto cleanup;
    }
    if (!reply)
    {
        _glfwInputError(GLFW_PLATFORM_ERROR,
                        "DBus: RetrieveFiles returned no reply");
        goto cleanup;
    }

    DBusMessageIter out, array;
    if (!dbus_message_iter_init(reply, &out))
    {
        _glfwInputError(GLFW_PLATFORM_ERROR,
                        "DBus: RetrieveFiles reply has no arguments");
        goto cleanup;
    }
    if (dbus_message_iter_get_arg_type(&out) != DBUS_TYPE_ARRAY ||
        dbus_message_iter_get_element_type(&out) != DBUS_TYPE_STRING)
    {
        _glfwInputError(GLFW_PLATFORM_ERROR,
                        "DBus: RetrieveFiles reply is not an array of strings");
        goto cleanup;
    }

    const int count = dbus_message_iter_get_element_count(&out);
    if (count == 0)
        goto cleanup;

    dbus_message_iter_recurse(&out, &array);
    paths = _glfw_calloc(count, sizeof(char*));
    if (!paths)
    {
        _glfwInputError(GLFW_OUT_OF_MEMORY, NULL);
        goto cleanup;
    }

    for (int i = 0; i < count; i++)
    {
        if (dbus_message_iter_get_arg_type(&array) != DBUS_TYPE_STRING)
        {
            _glfwInputError(GLFW_PLATFORM_ERROR,
                            "DBus: RetrieveFiles reply contains a non-string path");
            goto cleanup;
        }

        dbus_message_iter_get_basic(&array, &paths[i]);
        if (i + 1 < count && !dbus_message_iter_next(&array))
        {
            _glfwInputError(GLFW_PLATFORM_ERROR,
                            "DBus: RetrieveFiles reply ended unexpectedly");
            goto cleanup;
        }
    }

    _glfwInputDrop(window, count, (const char**) paths);

cleanup:
    _glfw_free(paths);
    if (reply)
        dbus_message_unref(reply);
    if (message)
        dbus_message_unref(message);
    if (connection)
        dbus_connection_unref(connection);
    if (dbus_error_is_set(&error))
        dbus_error_free(&error);
}

#endif // _GLFW_X11 || _GLFW_WAYLAND
