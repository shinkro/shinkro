import { useEffect, useState } from "react";
import { Flex, Group, Loader, Stack, Text, Button, Paper, Image } from "@mantine/core";
import { FaArrowRightArrowLeft } from "react-icons/fa6";
import { SiAnilist } from "react-icons/si";
import { APIClient } from "@api/APIClient.ts";
import Logo from "@app/logo.svg";

export const AnilistAuthCallback = () => {
    const [status, setStatus] = useState<"pending" | "success" | "error">("pending");
    const [errorMsg, setErrorMsg] = useState("");

    useEffect(() => {
        const params = new URLSearchParams(window.location.search);
        const code = params.get("code");
        const state = params.get("state");

        if (!code || !state) {
            setStatus("error");
            setErrorMsg("Missing code or state in URL");
            return;
        }

        APIClient.anilistauth.callback(code, state)
            .then(() => {
                setStatus("success");
                // Notify the opener (settings page) that auth completed
                window.opener?.postMessage({ type: "anilist-auth" }, window.location.origin);
            })
            .catch((err: Error) => {
                setStatus("error");
                setErrorMsg(err.message || "Authentication failed");
                window.opener?.postMessage({ type: "anilist-auth" }, window.location.origin);
            });
    }, []);

    return (
        <Flex
            direction={"column"}
            w={"100%"}
            maw={"600px"}
            miw={"280px"}
            mx={"auto"}
            pt={"10vh"}
            align={"stretch"}
        >
            <Paper withBorder p="md" shadow="xl">
                <Group justify={"center"}>
                    <Image src={Logo} fit="contain" h={80} />
                    <FaArrowRightArrowLeft size={50} />
                    <SiAnilist size={80} color={"#02a9ff"} />
                </Group>
                <Stack align="center" mt="md">
                    {status === "pending" && (
                        <>
                            <Loader size="xl" />
                            <Text>Authenticating with AniList...</Text>
                        </>
                    )}
                    {status === "success" && (
                        <Text size="xl" fw={600} c="green">Authentication Successful!</Text>
                    )}
                    {status === "error" && (
                        <>
                            <Text size="xl" fw={600} c="red">Authentication Failed</Text>
                            <Text c="dimmed" size="sm">{errorMsg}</Text>
                        </>
                    )}
                    <Text c={"dimmed"}>You may close this window now.</Text>
                    <Button onClick={() => window.close()}>CLOSE WINDOW</Button>
                </Stack>
            </Paper>
        </Flex>
    );
};
