"use client";

import React, { useState } from "react";
import { Form, Input, Button, Card, message, Space } from "antd";
import { LockOutlined, KeyOutlined, CheckCircleOutlined } from "@ant-design/icons";
import { ApiPost } from "@/lib/api";
import { useRouter } from "next/navigation";

export default function ChangePasswordPage() {
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm();
  const router = useRouter();

  const onFinish = async (values: any) => {
    setLoading(true);
    try {
      const resp = await ApiPost("/changePassword", {
        password: values.oldPassword,
        newPassword: values.newPassword,
      });

      if (resp.code === 0) {
        message.success("密码修改成功，请牢记您的新密码");
        form.resetFields();
      } else {
        message.error(resp.msg || "密码修改失败");
      }
    } catch (error) {
      console.error("Change password error:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ maxWidth: 600, margin: "24px auto" }}>
      <Card 
        title={
          <Space>
            <LockOutlined />
            <span>修改管理员密码</span>
          </Space>
        }
        variant="outlined"
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          autoComplete="off"
        >
          <Form.Item
            label="原密码"
            name="oldPassword"
            rules={[{ required: true, message: "请输入当前使用的密码" }]}
          >
            <Input.Password 
              prefix={<KeyOutlined style={{ color: "rgba(0,0,0,.25)" }} />} 
              placeholder="请输入原密码" 
            />
          </Form.Item>

          <Form.Item
            label="新密码"
            name="newPassword"
            rules={[
              { required: true, message: "请输入新密码" },
              { min: 6, message: "新密码长度不能少于6位" }
            ]}
          >
            <Input.Password 
              prefix={<LockOutlined style={{ color: "rgba(0,0,0,.25)" }} />} 
              placeholder="请输入新密码" 
            />
          </Form.Item>

          <Form.Item
            label="确认新密码"
            name="confirmPassword"
            dependencies={["newPassword"]}
            rules={[
              { required: true, message: "请再次输入新密码" },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue("newPassword") === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error("两次输入的新密码不一致"));
                },
              }),
            ]}
          >
            <Input.Password 
              prefix={<CheckCircleOutlined style={{ color: "rgba(0,0,0,.25)" }} />} 
              placeholder="请再次填写以确认" 
            />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0, marginTop: 12 }}>
            <Button type="primary" htmlType="submit" loading={loading} block size="large">
              确认修改
            </Button>
          </Form.Item>
        </Form>
      </Card>
      
      <div style={{ marginTop: 24, padding: "0 12px", color: "rgba(0,0,0,0.45)", fontSize: 13 }}>
        <p>提示：修改密码后无需重新登录。如果丢失密码，请联系数据库管理员手动重置。</p>
      </div>
    </div>
  );
}
