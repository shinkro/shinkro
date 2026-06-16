import {
    Button, Stack, Group, Grid, PasswordInput,
    ActionIcon, Text,
} from "@mantine/core";
import {CenteredEmptyState, SettingsSectionHeader} from "@screens/settings/components.tsx";
import {useSuspenseQuery, useQueryClient, useMutation} from "@tanstack/react-query";
import {ApikeysQueryOptions} from "@api/queries.ts";
import {displayNotification} from "@components/notifications";
import {useDisclosure} from "@mantine/hooks";
import {APIClient} from "@api/APIClient.ts";
import {ApiAddKey} from "@forms/settings/ApiAddKey.tsx";
import {FaCopy, FaCheck} from "react-icons/fa";
import {ConfirmDeleteIcon} from "@components/alerts/ConfirmDeleteIcon";
import {CopyTextToClipboard} from "@utils/index";
import {useState} from "react";

interface KeyFieldProps {
    value: string;
}

const KeyField = ({ value }: KeyFieldProps) => {
    const [isCopied, setIsCopied] = useState(false);

    const handleCopy = async () => {
        try {
            await CopyTextToClipboard(value);
            setIsCopied(true);
            setTimeout(() => setIsCopied(false), 1500);
            displayNotification({
                title: "Success",
                message: "API key copied to clipboard!",
                type: "success",
            });
        } catch (error) {
            console.error('Copy failed:', error);
            displayNotification({
                title: "Copy Failed",
                message: "Please manually copy the API key. Clipboard access may be restricted over HTTP.",
                type: "error",
            });
        }
    };

    return (
        <Group gap="xs" wrap="nowrap" style={{ width: '100%' }}>
            <PasswordInput
                value={value}
                readOnly
                variant="default"
                style={{ flex: 1 }}
            />
            <ActionIcon
                variant="outline"
                color={isCopied ? 'teal' : 'primary'}
                onClick={handleCopy}
            >
                {isCopied ? <FaCheck /> : <FaCopy />}
            </ActionIcon>
        </Group>
    );
};

export const Api = () => {
    const [opened, {open, close}] = useDisclosure(false);
    const {data: keys} = useSuspenseQuery(ApikeysQueryOptions());
    const queryClient = useQueryClient();

    const mutation = useMutation({
        mutationFn: (key: string) => APIClient.apikeys.delete(key),
        onSuccess: () => {
            queryClient.invalidateQueries(ApikeysQueryOptions());
            displayNotification({
                title: "API Key Deleted",
                type: "success",
            });
        },
    });

    return (
        <main>
            <SettingsSectionHeader
                title="API Keys"
                description="Manage your shinkro API keys here."
            />
            {keys?.length ? (
                <Stack mt="md" mb="md">
                    {/* Header row - hidden on mobile */}
                    <Grid
                        visibleFrom="sm"
                        gap="md"
                        mb="xs"
                        pb="xs"
                        style={{ borderBottom: '1px solid var(--mantine-color-gray-3)' }}
                    >
                        <Grid.Col span={{ base: 12, sm: 3 }}>
                            <Text
                                size="xs"
                                fw={700}
                                c="dimmed"
                                tt="uppercase"
                                style={{ letterSpacing: '0.05em', paddingLeft: '0.75rem' }}
                            >
                                Name
                            </Text>
                        </Grid.Col>
                        <Grid.Col span={{ base: 12, sm: 8 }}>
                            <Text
                                size="xs"
                                fw={700}
                                c="dimmed"
                                tt="uppercase"
                                style={{ letterSpacing: '0.05em', paddingLeft: '12rem' }}
                            >
                                Key
                            </Text>
                        </Grid.Col>
                    </Grid>

                    {/* API Key items */}
                    <Stack gap={0}>
                        {keys.map((keyItem) => (
                            <KeyRow 
                                key={keyItem.key} 
                                name={keyItem.name} 
                                value={keyItem.key} 
                                onDelete={mutation.mutate}
                            />
                        ))}
                    </Stack>
                </Stack>
            ) : (
                <CenteredEmptyState message="No API Keys Found"/>
            )}
            <Group justify="center" mt="md">
                <Button onClick={open} size="compact-xs">
                    ADD NEW
                </Button>
            </Group>
            <ApiAddKey opened={opened} onClose={close}/>
        </main>
    );
};

interface KeyRowProps {
    name: string;
    value: string;
    onDelete: (key: string) => void;
}

const KeyRow = ({name, value, onDelete}: KeyRowProps) => {
    return (
        <>
            <Grid gap="md" align="center" py="sm" style={{ borderBottom: '1px solid var(--mantine-color-gray-2)' }}>
                <Grid.Col span={{ base: 12, sm: 3 }}>
                    <Group justify="space-between" wrap="nowrap">
                        <Text c="primary" tt="uppercase" fw={700} truncate style={{ flex: 1 }}>
                            {name}
                        </Text>
                    </Group>
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 8 }}>
                    <KeyField value={value} />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 1 }} visibleFrom="sm">
                    <Group justify="center">
                        <ConfirmDeleteIcon
                            onConfirm={() => onDelete(value)}
                            title="Delete API Key"
                            message={`Are you sure you want to delete the API key "${name}"?`}
                            confirmText="DELETE"
                            variant="outline"
                        />
                    </Group>
                </Grid.Col>
            </Grid>
        </>
    );
};