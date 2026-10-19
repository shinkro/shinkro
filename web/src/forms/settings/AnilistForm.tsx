import { useForm } from "@mantine/form";
import { Button, Code, Group, Modal, PasswordInput, Text } from "@mantine/core";
import { AnilistAuth } from "@app/types/AnilistAuth";
import { useMutation } from "@tanstack/react-query";
import { APIClient } from "@api/APIClient.ts";
import { displayNotification } from "@components/notifications";
import { useEffect, useRef, useState } from "react";
import { CopyTextToClipboard } from "@utils/index";

interface Props {
    opened: boolean;
    onClose: () => void;
    loading: boolean;
    setLoading: (loading: boolean) => void;
}

export const AnilistForm = ({ opened, onClose, loading, setLoading }: Props) => {
    const [copied, setCopied] = useState(false);
    const [copyError, setCopyError] = useState(false);
    const isModalOpenRef = useRef(opened);

    // This must match exactly what the backend registers with AniList
    const redirectURL = `${window.location.origin}/anilistauth/callback`;

    const form = useForm<AnilistAuth>({
        initialValues: {
            clientID: "",
            clientSecret: "",
            redirectURL: redirectURL,
        },
        validate: {
            clientID: (value: string) => (value ? null : "Client ID is required"),
            clientSecret: (value: string) => (value ? null : "Client Secret is required"),
        },
    });

    const mutation = useMutation({
        mutationFn: APIClient.anilistauth.start,
        onSuccess: (data) => {
            if (!isModalOpenRef.current) return;
            const url = data?.url;
            if (url) {
                setLoading(true);
                window.open(url, "_blank");
            }
        },
        onError: (error) => {
            if (!isModalOpenRef.current) return;
            displayNotification({
                title: "AniList Authentication Failed",
                message: error.message || "Could not start AniList authentication",
                type: "error",
            });
        },
    });

    useEffect(() => {
        isModalOpenRef.current = opened;
        if (!opened) {
            mutation.reset();
            setLoading(false);
        }
    }, [opened, mutation, setLoading]);

    const handleFormSubmit = (values: AnilistAuth) => {
        mutation.mutate(values);
        form.reset();
    };

    const handleCopy = async () => {
        try {
            await CopyTextToClipboard(redirectURL);
            setCopied(true);
            setCopyError(false);
            setTimeout(() => setCopied(false), 2000);
        } catch {
            setCopyError(true);
            displayNotification({
                title: "Copy Failed",
                message: "Please manually copy the URL. Clipboard access may be restricted over HTTP.",
                type: "info",
            });
        }
    };

    return (
        <Modal opened={opened} onClose={onClose} title={"Login to AniList"}>
            <form onSubmit={form.onSubmit(handleFormSubmit)}>
                <PasswordInput
                    label="Client ID"
                    placeholder="Enter Client ID"
                    {...form.getInputProps("clientID")}
                />
                <PasswordInput
                    label="Client Secret"
                    placeholder="Enter Client Secret"
                    {...form.getInputProps("clientSecret")}
                    mt={"md"}
                />
                <Group mt={"md"}>
                    <Text size="sm">Redirect URL (paste this in AniList)</Text>
                    <Code c="dimmed">{redirectURL}</Code>
                </Group>
                <Group justify={"center"} align={"flex-end"}>
                    <Button
                        color={copied ? "teal" : copyError ? "red" : "blue"}
                        onClick={handleCopy}
                    >
                        {copied ? "COPIED URL" : copyError ? "COPY FAILED" : "COPY REDIRECT URL"}
                    </Button>
                    <Button type="submit" mt={"md"} loading={loading}>
                        LOGIN
                    </Button>
                </Group>
            </form>
        </Modal>
    );
};
