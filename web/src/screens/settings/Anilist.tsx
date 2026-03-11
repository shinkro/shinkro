import { APIClient } from "@api/APIClient.ts";
import { displayNotification } from "@components/notifications";
import { useState, useEffect } from "react";
import { Stack, Button, Group } from "@mantine/core";
import { useMutation, useSuspenseQuery, useQueryClient } from "@tanstack/react-query";
import { AnilistQueryOptions } from "@api/queries.ts";
import { AnilistAuthKeys } from "@api/query_keys.ts";
import { ConfirmDeleteButton } from "@components/alerts/ConfirmDeleteButton";
import { useDisclosure } from "@mantine/hooks";
import { AnilistForm } from "@forms/settings/AnilistForm.tsx";
import { CenteredEmptyState, SettingsSectionHeader, StatusIndicator } from "@screens/settings/components.tsx";

export const Anilist = () => {
    const queryClient = useQueryClient();
    const { data: anilistauth } = useSuspenseQuery(AnilistQueryOptions());
    const [loading, setLoading] = useState(false);
    const [opened, { open, close }] = useDisclosure(false);
    const [testSuccess, setTestSuccess] = useState<boolean | null>(null);

    const isEmptySettings = !anilistauth || Object.keys(anilistauth).length === 0;

    useEffect(() => {
        if (!isEmptySettings) {
            APIClient.anilistauth.test()
                .then(() => setTestSuccess(true))
                .catch(() => setTestSuccess(false));
        } else {
            setTestSuccess(null);
        }
    }, [anilistauth]);

    useEffect(() => {
        const handleMessage = (event: MessageEvent) => {
            if (event.origin !== window.location.origin) return;
            if (event.data?.type === "anilist-auth") {
                queryClient.invalidateQueries({ queryKey: AnilistAuthKeys.config() });
                setLoading(false);
                close();
            }
        };
        window.addEventListener("message", handleMessage);
        return () => window.removeEventListener("message", handleMessage);
    }, [queryClient]);

    const deleteMutation = useMutation({
        mutationFn: APIClient.anilistauth.delete,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: AnilistAuthKeys.config() });
            displayNotification({
                title: "AniList Authentication Deleted Successfully",
                message: "AniList updates will no longer be sent",
                type: "success",
            });
        },
        onError: (error) => {
            displayNotification({
                title: "Deletion Failed",
                message: error.message || "Could not delete AniList authentication",
                type: "error",
            });
        },
    });

    return (
        <main>
            <SettingsSectionHeader
                title={"AniList"}
                description={"Manage the connection to your AniList account here."}
            />
            {isEmptySettings ? (
                <CenteredEmptyState
                    message={"No AniList Credentials Found"}
                    button={
                        <Button onClick={open}>
                            START AUTHENTICATION
                        </Button>
                    }
                />
            ) : (
                <Stack mt={"md"}>
                    <StatusIndicator
                        label={"Authentication Status:"}
                        status={testSuccess}
                        loadStatus={testSuccess === null}
                    />
                    <Group justify="flex-start">
                        <ConfirmDeleteButton
                            message={"AniList credentials will be deleted."}
                            confirmText={"REMOVE ACCESS"}
                            onConfirm={() => deleteMutation.mutate()}
                        />
                        <Button onClick={open}>RE - AUTHENTICATE</Button>
                    </Group>
                </Stack>
            )}
            <AnilistForm
                opened={opened}
                onClose={close}
                loading={loading}
                setLoading={setLoading}
            />
        </main>
    );
};
