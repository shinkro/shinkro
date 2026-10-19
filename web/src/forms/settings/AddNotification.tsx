import {useMutation, useQueryClient} from "@tanstack/react-query";
import {useForm} from "@mantine/form";
import {Modal, TextInput, Select, Switch, MultiSelect, Button, Stack, Group} from "@mantine/core";
import {APIClient} from "@api/APIClient.ts";
import {NotificationKeys} from "@api/query_keys.ts";
import {displayNotification} from "@components/notifications";
import {useEffect} from "react";

interface AddNotificationProps {
    opened: boolean;
    onClose: () => void;
    defaultValues?: Partial<ServiceNotification>;
}

export const AddNotification = ({opened, onClose, defaultValues}: AddNotificationProps) => {
    const queryClient = useQueryClient();
    
    const form = useForm<ServiceNotification>({
        initialValues: {
            id: 0,
            name: "",
            enabled: true,
            type: "DISCORD",
            events: [],
        },
        validate: {
            name: (value) => (value ? null : "Required"),
            type: (value) => (value ? null : "Required"),
            webhook: (value, values) => 
                values.type === "DISCORD" && !value ? "Webhook URL is required for Discord" : null,
            token: (value, values) => 
                values.type === "GOTIFY" && !value ? "Token is required for Gotify" : null,
            host: (value, values) => 
                values.type === "GOTIFY" && !value ? "Host is required for Gotify" : null,
        },
    });

    useEffect(() => {
        if (defaultValues && Object.keys(defaultValues).length !== 0) {
            form.setValues(defaultValues);
        } else {
            form.reset();
        }
    }, [defaultValues]);

    const createMutation = useMutation({
        mutationFn: (notification: ServiceNotification) => APIClient.notifications.create(notification),
        onSuccess: (data: ServiceNotification) => {
            queryClient.invalidateQueries({queryKey: NotificationKeys.lists()});
            displayNotification({
                title: "Notification Created",
                message: `Notification ${data.name} was created`,
                type: "success",
            });
            onClose();
        },
    });

    const updateMutation = useMutation({
        mutationFn: (notification: ServiceNotification) => APIClient.notifications.update(notification),
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: NotificationKeys.lists()});
            displayNotification({
                title: "Notification Updated",
                message: `Notification ${form.values.name} was updated`,
                type: "success",
            });
            onClose();
        },
    });

    const testMutation = useMutation({
        mutationFn: (notification: ServiceNotification) => APIClient.notifications.test(notification),
        onSuccess: () => {
            displayNotification({
                title: "Test Successful",
                message: "Test notification was sent successfully",
                type: "success",
            });
        },
        onError: (error: Error) => {
            displayNotification({
                title: "Test Failed",
                message: error.message,
                type: "error",
            });
        },
    });

    const handleSubmit = (values: typeof form.values) => {
        const isEditing = !!defaultValues?.id;
        if (isEditing) {
            updateMutation.mutate(values as ServiceNotification);
        } else {
            createMutation.mutate(values as ServiceNotification);
        }
    };

    const handleTest = () => {
        const values = form.values;
        if (form.validate().hasErrors) {
            displayNotification({
                title: "Validation Error",
                message: "Please fix the form errors before testing",
                type: "error",
            });
            return;
        }
        testMutation.mutate(values as ServiceNotification);
    };

    const notificationEvents = [
        { value: "APP_UPDATE_AVAILABLE", label: "App Update Available" },
        { value: "SUCCESS", label: "Anime Update Successful" },
        { value: "PLEX_PROCESSING_ERROR", label: "Plex Processing Error" },
        { value: "ANIME_UPDATE_ERROR", label: "Anime Update Error" },
        { value: "TEST", label: "Test" },
    ];

    const isEditing = !!defaultValues?.id;

    return (
        <Modal opened={opened} onClose={onClose} title={isEditing ? "Edit Notification" : "Add Notification"}>
            <form onSubmit={form.onSubmit(handleSubmit)}>
                <Stack>
                    <TextInput
                        label="Name"
                        placeholder="Enter notification name"
                        {...form.getInputProps("name")}
                    />
                    
                    <Select
                        label="Type"
                        placeholder="Select notification type"
                        data={[
                            { value: "DISCORD", label: "Discord" },
                            { value: "GOTIFY", label: "Gotify" },
                        ]}
                        {...form.getInputProps("type")}
                    />

                    <Switch
                        label="Enabled"
                        {...form.getInputProps("enabled", { type: "checkbox" })}
                    />

                    <MultiSelect
                        label="Events"
                        placeholder="Select events to notify"
                        data={notificationEvents}
                        searchable
                        clearable
                        {...form.getInputProps("events")}
                    />

                    {form.values.type === "DISCORD" && (
                        <TextInput
                            label="Webhook URL"
                            placeholder="Enter Discord webhook URL"
                            {...form.getInputProps("webhook")}
                        />
                    )}

                    {form.values.type === "GOTIFY" && (
                        <>
                            <TextInput
                                label="Host"
                                placeholder="Enter Gotify server URL"
                                {...form.getInputProps("host")}
                            />
                            <TextInput
                                label="Token"
                                placeholder="Enter Gotify application token"
                                {...form.getInputProps("token")}
                            />
                            <TextInput
                                label="Priority"
                                type="number"
                                placeholder="Enter notification priority (1-10)"
                                {...form.getInputProps("priority")}
                            />
                        </>
                    )}

                    <Group justify="flex-end" mt="md">
                        <Button variant="default" onClick={onClose}>
                            Cancel
                        </Button>
                        <Button 
                            variant="outline" 
                            onClick={handleTest}
                            loading={testMutation.isPending}
                        >
                            Test
                        </Button>
                        <Button type="submit" loading={isEditing ? updateMutation.isPending : createMutation.isPending}>
                            {isEditing ? "Update" : "Save"}
                        </Button>
                    </Group>
                </Stack>
            </form>
        </Modal>
    );
}